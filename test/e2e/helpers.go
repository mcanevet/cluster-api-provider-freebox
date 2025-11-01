//go:build e2e
// +build e2e

/*
Copyright 2025.

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

package e2e

import (
	"context"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

// GetFreeboxMachineInput is the input for GetFreeboxMachine.
type GetFreeboxMachineInput struct {
	Getter    client.Client
	Name      string
	Namespace string
}

// GetFreeboxMachine gets a FreeboxMachine object.
func GetFreeboxMachine(ctx context.Context, input GetFreeboxMachineInput, intervals ...interface{}) *infrastructurev1alpha1.FreeboxMachine {
	machine := &infrastructurev1alpha1.FreeboxMachine{}
	key := types.NamespacedName{
		Name:      input.Name,
		Namespace: input.Namespace,
	}
	Eventually(func() error {
		return input.Getter.Get(ctx, key, machine)
	}, intervals...).Should(Succeed(), "Failed to get FreeboxMachine %s/%s", input.Namespace, input.Name)
	return machine
}

// WaitForFreeboxMachineDeletedInput is the input for WaitForFreeboxMachineDeleted.
type WaitForFreeboxMachineDeletedInput struct {
	Getter  client.Client
	Machine *infrastructurev1alpha1.FreeboxMachine
}

// WaitForFreeboxMachineDeleted waits for a FreeboxMachine to be deleted.
func WaitForFreeboxMachineDeleted(ctx context.Context, input WaitForFreeboxMachineDeletedInput, intervals ...interface{}) {
	key := types.NamespacedName{
		Name:      input.Machine.Name,
		Namespace: input.Machine.Namespace,
	}
	Eventually(func() bool {
		machine := &infrastructurev1alpha1.FreeboxMachine{}
		err := input.Getter.Get(ctx, key, machine)
		return err != nil
	}, intervals...).Should(BeTrue(), "FreeboxMachine %s/%s was not deleted", input.Machine.Namespace, input.Machine.Name)
}

// GetObjectKey returns the ObjectKey for a client.Object.
func GetObjectKey(obj client.Object) types.NamespacedName {
	return types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}
