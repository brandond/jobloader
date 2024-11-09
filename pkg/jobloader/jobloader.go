package jobloader

import (
	"context"

	"github.com/brandond/jobloader/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	toolswatch "k8s.io/client-go/tools/watch"
	"k8s.io/utils/pointer"
)

var labelJobLoader = "jobloader.khaus.io/owned"

type JobLoader struct {
	Kubeconfig  string
	JobsPerNode int64
	Namespace   string
	Debug       bool

	ctx    context.Context
	client kubernetes.Interface
}

func (j *JobLoader) Run(_ *cli.Context) error {
	j.ctx = signals.SetupSignalContext()

	if j.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", j.Kubeconfig)
	if err != nil {
		return err
	}

	j.client, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	ver, err := j.client.Discovery().ServerVersion()
	if err != nil {
		return err
	}
	logrus.Infof("Connected to Kubernetes %s", ver)

	return j.doJobs()
}

func (j *JobLoader) doJobs() error {
	nodeCount, err := j.getNodeCount()
	if err != nil {
		return err
	}
	logrus.Infof("Creating %d jobs each for %d nodes", j.JobsPerNode, nodeCount)

	go j.createReplacementJobs()

	for i := int64(0); i < j.JobsPerNode*nodeCount; i++ {
		j.createJob()
	}

	<-j.ctx.Done()
	return j.ctx.Err()
}

func (j *JobLoader) createJob() {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "jobloader-",
			Labels: map[string]string{
				labelJobLoader: "true",
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism:             pointer.Int32(1),
			TTLSecondsAfterFinished: pointer.Int32(5),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelJobLoader: "true",
					},
				},
				Spec: v1.PodSpec{
					RestartPolicy:                v1.RestartPolicyNever,
					EnableServiceLinks:           pointer.Bool(false),
					AutomountServiceAccountToken: pointer.Bool(false),
					Containers: []v1.Container{
						{
							Name:    "httpd",
							Image:   "docker.io/rancher/mirrored-library-busybox:1.36.1",
							Command: []string{"sh", "-c"},
							Args:    []string{"sleep $(expr $RANDOM % 30); echo ok > /tmp/index.html; httpd -vv -p 8080 -h /tmp; sleep 60; sleep $(expr $RANDOM % 30); killall httpd; true"},
							ReadinessProbe: &v1.Probe{
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(8080),
									},
								},
							},
						},
					},
					TopologySpreadConstraints: []v1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       "kubernetes.io/hostname",
							WhenUnsatisfiable: v1.ScheduleAnyway,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									labelJobLoader: "true",
								},
							},
						},
					},
				},
			},
		},
	}
	if job, err := j.client.BatchV1().Jobs(j.Namespace).Create(j.ctx, job, metav1.CreateOptions{}); err != nil {
		logrus.Errorf("Failed to create Job: %v", err)
	} else {
		logrus.Errorf("Created Job: %s", job.Name)
	}
}

func (j *JobLoader) createReplacementJobs() {
	watcher, done := j.getJobWatch()

	defer func() {
		logrus.Infof("Terminating watch...")
		watcher.Stop()
		<-done
		logrus.Infof("Done.")
	}()

	logrus.Infof("Starting watch...")
	for {
		select {
		case <-j.ctx.Done():
			return
		case ev, ok := <-watcher.ResultChan():
			job, ok := ev.Object.(*batchv1.Job)
			if !ok {
				logrus.Errorf("Watch error: event object not of type batchv1.Job")
				continue
			}
			if ev.Type == watch.Deleted {
				logrus.Infof("Deleted Job: %s", job.Name)
				j.createJob()
			}
		}
	}
}

func (j *JobLoader) getNodeCount() (int64, error) {
	nodeList, err := j.client.CoreV1().Nodes().List(j.ctx, metav1.ListOptions{Limit: 1})
	var count int64
	if nodeList != nil {
		count = int64(len(nodeList.Items))
		if nodeList.RemainingItemCount != nil {
			count += *nodeList.RemainingItemCount
		}
	}
	return count, err
}

func (j *JobLoader) getJobWatch() (watch.Interface, <-chan struct{}) {
	jobs := j.client.BatchV1().Jobs(j.Namespace)
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
			options.LabelSelector = labelJobLoader
			return jobs.List(j.ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.LabelSelector = labelJobLoader
			return jobs.Watch(j.ctx, options)
		},
	}

	_, _, watch, done := toolswatch.NewIndexerInformerWatcher(lw, &batchv1.Job{})
	return watch, done
}
