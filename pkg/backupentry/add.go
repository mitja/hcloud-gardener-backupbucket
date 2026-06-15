// SPDX-FileCopyrightText: 2026 Mitja Martini
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Type is the BackupEntry provider type this extension claims.
const Type = "hcloud"

// DefaultAddOptions are the default AddOptions for AddToManager.
var DefaultAddOptions = AddOptions{}

// AddOptions are the options to apply when adding the BackupEntry controller.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation.
	IgnoreOperationAnnotation bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// AddToManager adds the BackupEntry controller with the default options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}

// AddToManagerWithOptions adds the BackupEntry controller with the given options.
func AddToManagerWithOptions(_ context.Context, mgr manager.Manager, opts AddOptions) error {
	return backupentry.Add(mgr, backupentry.AddArgs{
		Actuator:                  genericactuator.NewActuator(mgr, newDelegate(mgr)),
		ControllerOptions:         opts.Controller,
		Predicates:                backupentry.DefaultPredicates(opts.IgnoreOperationAnnotation),
		Type:                      Type,
		IgnoreOperationAnnotation: opts.IgnoreOperationAnnotation,
		ExtensionClass:            opts.ExtensionClass,
	})
}
