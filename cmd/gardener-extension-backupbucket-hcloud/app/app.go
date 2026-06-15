// SPDX-FileCopyrightText: 2026 Mitja Martini
//
// SPDX-License-Identifier: Apache-2.0

// Package app wires up the gardener-extension-backupbucket-hcloud manager and its
// controllers (BackupBucket + BackupEntry for Hetzner Object Storage).
package app

import (
	"context"
	"fmt"

	backupbucketctrl "github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	backupentryctrl "github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	extensionscmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	heartbeatcmd "github.com/gardener/gardener/extensions/pkg/controller/heartbeat/cmd"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	hcloudbackupbucket "github.com/mitja/hcloud-gardener-backupbucket/pkg/backupbucket"
	hcloudbackupentry "github.com/mitja/hcloud-gardener-backupbucket/pkg/backupentry"
)

// ExtensionName is the name of the extension.
const ExtensionName = "gardener-extension-backupbucket-hcloud"

// Options holds the command's aggregated options.
type Options struct {
	generalOptions     *extensionscmd.GeneralOptions
	restOptions        *extensionscmd.RESTOptions
	managerOptions     *extensionscmd.ManagerOptions
	controllerOptions  *extensionscmd.ControllerOptions
	heartbeatOptions   *heartbeatcmd.Options
	reconcileOptions   *extensionscmd.ReconcilerOptions
	controllerSwitches *extensionscmd.SwitchOptions
	optionAggregator   extensionscmd.OptionAggregator
}

// NewOptions creates a new Options instance with the default configuration.
func NewOptions() *Options {
	opts := &Options{
		generalOptions: &extensionscmd.GeneralOptions{},
		restOptions:    &extensionscmd.RESTOptions{},
		managerOptions: &extensionscmd.ManagerOptions{
			LeaderElection:          true,
			LeaderElectionID:        extensionscmd.LeaderElectionNameID(ExtensionName),
			LeaderElectionNamespace: "kube-system",
			WebhookServerPort:       443,
			MetricsBindAddress:      ":8080",
			HealthBindAddress:       ":8081",
		},
		controllerOptions: &extensionscmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		},
		heartbeatOptions: &heartbeatcmd.Options{
			ExtensionName:        ExtensionName,
			RenewIntervalSeconds: 30,
		},
		reconcileOptions: &extensionscmd.ReconcilerOptions{},
		controllerSwitches: extensionscmd.NewSwitchOptions(
			extensionscmd.Switch(backupbucketctrl.ControllerName, hcloudbackupbucket.AddToManager),
			extensionscmd.Switch(backupentryctrl.ControllerName, hcloudbackupentry.AddToManager),
			extensionscmd.Switch(heartbeat.ControllerName, heartbeat.AddToManager),
		),
	}

	opts.optionAggregator = extensionscmd.NewOptionAggregator(
		opts.generalOptions,
		opts.restOptions,
		opts.managerOptions,
		opts.controllerOptions,
		extensionscmd.PrefixOption("heartbeat-", opts.heartbeatOptions),
		opts.reconcileOptions,
		opts.controllerSwitches,
	)

	return opts
}

// NewCommand creates the root cobra command for the extension.
func NewCommand() *cobra.Command {
	opts := NewOptions()

	cmd := &cobra.Command{
		Use:   ExtensionName,
		Short: "BackupBucket/BackupEntry controller for Hetzner Object Storage (provider type hcloud)",

		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.optionAggregator.Complete(); err != nil {
				return fmt.Errorf("error completing options: %w", err)
			}
			return opts.run(cmd.Context())
		},
	}

	opts.optionAggregator.AddFlags(cmd.Flags())
	return cmd
}

func (o *Options) run(ctx context.Context) error {
	mgr, err := manager.New(o.restOptions.Completed().Config, o.managerOptions.Completed().Options())
	if err != nil {
		return fmt.Errorf("could not create manager: %w", err)
	}

	if err := extensionsv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return fmt.Errorf("could not register extensions scheme: %w", err)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return fmt.Errorf("could not add healthz check: %w", err)
	}
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		return fmt.Errorf("could not add readyz check: %w", err)
	}

	controllerCfg := o.controllerOptions.Completed()
	controllerCfg.Apply(&hcloudbackupbucket.DefaultAddOptions.Controller)
	controllerCfg.Apply(&hcloudbackupentry.DefaultAddOptions.Controller)

	o.heartbeatOptions.Completed().Apply(&heartbeat.DefaultAddOptions)

	reconcileCfg := o.reconcileOptions.Completed()
	reconcileCfg.Apply(&hcloudbackupbucket.DefaultAddOptions.IgnoreOperationAnnotation, &hcloudbackupbucket.DefaultAddOptions.ExtensionClass)
	reconcileCfg.Apply(&hcloudbackupentry.DefaultAddOptions.IgnoreOperationAnnotation, &hcloudbackupentry.DefaultAddOptions.ExtensionClass)

	if err := o.controllerSwitches.Completed().AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("could not add controllers to manager: %w", err)
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("error running manager: %w", err)
	}
	return nil
}

// Run is the entry point used by main: it builds and executes the command,
// installing the controller-runtime signal handler as the root context.
func Run() error {
	cmd := NewCommand()
	cmd.SetContext(signals.SetupSignalHandler())
	return cmd.Execute()
}
