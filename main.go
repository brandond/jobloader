package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/brandond/jobloader/app"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
	})

	jobloader := app.NewJobLoader()
	if err := jobloader.Run(os.Args); err != nil {
		if !errors.Is(err, context.Canceled) {
			logrus.Fatal(err)
		}
	}
}
