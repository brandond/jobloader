package app

import (
	"fmt"

	"github.com/brandond/jobloader/pkg/jobloader"
	"github.com/brandond/jobloader/pkg/version"
	"github.com/urfave/cli/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewJobLoader() *cli.App {
	j := &jobloader.JobLoader{}
	return &cli.App{
		Name:            "jobloader",
		Usage:           "Load a Kubernetes cluster with dumb Jobs",
		Action:          j.Run,
		Version:         fmt.Sprintf("%s (%.8s)", version.GitVersion, version.GitCommit),
		HideHelpCommand: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "kubeconfig",
				EnvVars:     []string{"KUBECONFIG"},
				Destination: &j.Kubeconfig,
			},
			&cli.StringFlag{
				Name:        "namespace",
				Destination: &j.Namespace,
				Value:       metav1.NamespaceDefault,
			},
			&cli.Int64Flag{
				Name:        "jobs-per-node",
				Destination: &j.JobsPerNode,
				Value:       100,
			},
			&cli.BoolFlag{
				Name:        "debug",
				Destination: &j.Debug,
			},
		},
	}
}
