# jobloader - Load a Kubernetes cluster with dumb Jobs

Creates a bunch of Jobs running busybox.
The Pods have readiness checks that fail for a bit before passing.
They run for a minute or so doing mostly nothing and then exit.
As soon as a Job is done and deleted, a new one is created to replace it.

The main idea is to give the datastore/apiserver/controller-manager/scheduler a fair bit of work to do without loading the actual nodes too much.
Its not perfect or scientific but it does something.

## Help
```
NAME:
   jobloader - Load a Kubernetes cluster with dumb Jobs

USAGE:
   jobloader [global options]

GLOBAL OPTIONS:
   --kubeconfig value      [$KUBECONFIG]
   --namespace value      (default: "default")
   --jobs-per-node value  (default: 100)
   --debug                (default: false)
   --help, -h             show help
```
