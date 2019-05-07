/*
Copyright 2019 Banzai Cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/banzaicloud/kafka-operator/internal"
	"github.com/banzaicloud/kafka-operator/pkg/apis"
	"github.com/banzaicloud/kafka-operator/pkg/controller"
	"github.com/banzaicloud/kafka-operator/pkg/webhook"
	"github.com/oklog/run"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func main() {
	var metricsAddr, receiverAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&receiverAddr, "receiver-addr", ":9001", "The address the receiver endpoint binds to.")
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))
	log := logf.Log.WithName("entrypoint")

	// Get a config to talk to the apiserver
	log.Info("setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	log.Info("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: metricsAddr})
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	log.Info("setting up scheme")
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add APIs to scheme")
		os.Exit(1)
	}

	// Setup all Controllers
	log.Info("Setting up controller")
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "unable to register controllers to the manager")
		os.Exit(1)
	}

	log.Info("setting up webhooks")
	if err := webhook.AddToManager(mgr); err != nil {
		log.Error(err, "unable to register webhooks to the manager")
		os.Exit(1)
	}

	var g run.Group
	{
		ln, _ := net.Listen("tcp", receiverAddr)
		httpServer := &http.Server{Handler: internal.NewApp(log)}

		g.Add(
			func() error {
				log.Info("Starting the HTTP server.")
				return httpServer.Serve(ln)
			},
			func(e error) {
				log.Info("shutting server down")

				ctx := context.Background()
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
				defer cancel()

				err := httpServer.Shutdown(ctx)
				log.Error(err, "error")

				_ = httpServer.Close()
			},
		)
	}

	{
		var (
			cancelInterrupt = make(chan struct{})
			ch              = make(chan os.Signal, 2)
		)
		defer close(ch)

		g.Add(
			func() error {
				signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

				select {
				case sig := <-ch:
					log.Info("captured signal", map[string]interface{}{"signal": sig})
				case <-cancelInterrupt:
				}

				return nil
			},
			func(e error) {
				close(cancelInterrupt)
				signal.Stop(ch)
			},
		)
	}

	{
		// Start the Cmd
		g.Add(
			func() error {
				log.Info("Starting the Cmd.")
				return mgr.Start(signals.SetupSignalHandler())
			},
			func(error) {
				log.Error(err, "unable to run the manager")
				os.Exit(1)
			},
		)
	}

	g.Run()
}
