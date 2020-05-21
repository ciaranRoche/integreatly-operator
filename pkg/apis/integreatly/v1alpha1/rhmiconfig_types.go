/*
Copyright YEAR Red Hat, Inc.

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

package v1alpha1

import (
	"errors"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RHMIConfigSpec defines the desired state of RHMIConfig
type RHMIConfigSpec struct {
	Upgrade     Upgrade     `json:"upgrade,omitempty"`
	Maintenance Maintenance `json:"maintenance,omitempty"`
	Backup      Backup      `json:"backup,omitempty"`
}

// RHMIConfigStatus defines the observed state of RHMIConfig
type RHMIConfigStatus struct {
	// status block reflects the current configuration of the cr
	//
	//	status:
	//		maintenance:
	//			apply-from: 16-05-2020 23:00
	//			duration: "6hrs"
	//		upgrade:
	//			window: "3 Jan 1980 - 17 Jan 1980"
	Maintenance RHMIConfigStatusMaintenance `json:"maintenance,omitempty"`
	Upgrade     RHMIConfigStatusUpgrade     `json:"upgrade,omitempty"`
}

type RHMIConfigStatusMaintenance struct {
	ApplyFrom string `json:"applyFrom,omitempty"`
	Duration  string `json:"duration,omitempty"`
}

type RHMIConfigStatusUpgrade struct {
	Window string `json:"window,omitempty"`
}

const DateFormat = "2 Jan 2006 15:04"

type Upgrade struct {
	// contacts: list of contacts which are comma separated
	// "user1@example.com,user2@example.com"
	Contacts string `json:"contacts,omitempty"`
	// always-immediately: boolean value, if set to true an upgrade will be applied as soon as it is available,
	// whether service affecting or not.
	// This takes precedences over all other options
	AlwaysImmediately bool `json:"alwaysImmediately"`
	// during-next-maintenance: boolean value, if set to true an upgrade will be applied within the next maintenance window.
	// Takes precedence over apply-on
	DuringNextMaintenance bool `json:"duringNextMaintenance"`
	// apply-on: string date value. If 'always-immediately' or 'during-next-maintenance' is not set the customer is
	// required to pick a time for the upgrade. Time value will be validated by a webhook and reset to blank after
	// upgrade has completed. Format: "dd MMM YYYY hh:mm" > "12 Jan 1980 23:00". UTC time
	ApplyOn string `json:"applyOn,omitempty"`
}

type Maintenance struct {
	// apply-from: string, day time. Currently this is a 6 hour window.
	// Format: "DDD hh:mm" > "sun 23:00". UTC time
	ApplyFrom string `json:"applyFrom,omitempty"`
}

type Backup struct {
	// apply-on: string, day time.
	// Format: "DDD hh:mm" > "wed 20:00". UTC time
	ApplyOn string `json:"applyOn,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RHMIConfig is the Schema for the rhmiconfigs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=rhmiconfigs,scope=Namespaced
type RHMIConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RHMIConfigSpec   `json:"spec,omitempty"`
	Status RHMIConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RHMIConfigList contains a list of RHMIConfig
type RHMIConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RHMIConfig `json:"items"`
}

func (c *RHMIConfig) ValidateCreate() error {
	return nil
}

func (c *RHMIConfig) ValidateUpdate(old runtime.Object) error {
	if _, _, err := ValidateBackupAndMaintenance(c.Spec.Backup.ApplyOn, c.Spec.Maintenance.ApplyFrom); err != nil {
		return err
	}

	if c.Spec.Upgrade.ApplyOn == "" {
		return nil
	}

	if c.Spec.Upgrade.AlwaysImmediately || c.Spec.Upgrade.DuringNextMaintenance {
		return errors.New("spec.Upgrade.ApplyOn shouldn't be set when spec.Upgrade.AlwaysImmediatly or spec.Upgrade.DuringNextMaintenance are true")
	}

	applyOn, err := time.Parse(DateFormat, c.Spec.Upgrade.ApplyOn)
	if err != nil {
		return fmt.Errorf("invalid value for spec.Upgrade.ApplyOn, must be a date with the format %s", DateFormat)
	}

	if !applyOn.UTC().After(time.Now().UTC()) {
		return fmt.Errorf("invalid value for spec.Upgrade.ApplyOn: %s. It must be a future date", applyOn.Format(DateFormat))
	}

	return nil
}

func (c *RHMIConfig) ValidateDelete() error {
	return nil
}

func init() {
	SchemeBuilder.Register(&RHMIConfig{}, &RHMIConfigList{})
}

// we require backup and maintenance windows to be a minimum of 1hour block each
// these blocks can not over lap each other
// this function checks that the applyOn and applyFrom values are correctly formatted
// it builds 1 hour blocks and ensures these times do not overlap
func ValidateBackupAndMaintenance(backupApplyOn, maintenanceApplyFrom string) (string, string, error) {
	// we should expect both values to be set when RHMI config is updated
	if maintenanceApplyFrom == "" {
		return "", "", errors.New(fmt.Sprintf("maintenance apply from value is required"))
	}
	if backupApplyOn == "" {
		return "", "", errors.New(fmt.Sprintf("backup apply on value is required"))
	}

	// ensure backup applyOn format is correct
	parsedBackupTime, err := time.Parse("15:04", backupApplyOn)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse backup ApplyOn value : expected format HH:mm : %v", err)
	}

	// ensure maintenance applyFrom format is correct
	// split maintenance value expected format : `DDD HH:mm`
	windowSegments := strings.Split(maintenanceApplyFrom, " ")
	// check if day is correct
	var expectedDays = []string{
		"sun",
		"mon",
		"tue",
		"wed",
		"thu",
		"fri",
		"sat",
	}
	if !contains(expectedDays, strings.ToLower(windowSegments[0])) {
		return "", "", errors.New(fmt.Sprintf("formating failure, applyFrom format expected `DDD HH:mm` found : %s", maintenanceApplyFrom))
	}
	// check if maintenance applyFrom time value is correct
	parsedMaintenanceTime, err := time.Parse("15:04", windowSegments[1])
	if err != nil {
		return "", "", fmt.Errorf("failure while parsing maintenance applyFrom value, format expected `DDD HH:mm` found : %s : %v", maintenanceApplyFrom, err)
	}

	// we require a minimum of 1hr windows for both maintenance and backup
	// these windows can not over lap, this is a requirement of AWS
	// set maintenance time plus one hour
	parsedMaintenanceTimePlusOneHour := parsedMaintenanceTime.Add(time.Hour)
	// add one hour to applyOn format
	parsedBackupTimePlusOneHour := parsedBackupTime.Add(time.Hour)

	// build expected maintenance window strings for error message,
	builtMaintenanceString := fmt.Sprintf("%02d:%02d-%02d:%02d", parsedMaintenanceTime.Hour(), parsedMaintenanceTime.Minute(), parsedMaintenanceTimePlusOneHour.Hour(), parsedMaintenanceTimePlusOneHour.Minute())
	builtBackupString := fmt.Sprintf("%02d:%02d-%02d:%02d", parsedBackupTime.Hour(), parsedBackupTime.Minute(), parsedBackupTimePlusOneHour.Hour(), parsedBackupTimePlusOneHour.Minute())

	// ensure backup and maintenance time ranges do not overlap
	// we expect RHMI operator to validate the ranges, as a sanity check we preform an extra validation here
	// this is to avoid an obscure error message from AWS when we apply the times
	// http://baodad.blogspot.com/2014/06/date-range-overlap.html
	// (StartA <= EndB)  and  (EndA >= StartB)
	if timeBlockOverlaps(parsedBackupTime, parsedBackupTimePlusOneHour, parsedMaintenanceTime, parsedMaintenanceTimePlusOneHour) {
		return "", "", errors.New(fmt.Sprintf("backup and maintenance times can not overlap, each time is parsed as a 1 hour window, current backup applyOn window : %s overlaps with current maintenance window : %s ", builtBackupString, builtMaintenanceString))
	}
	return backupApplyOn, maintenanceApplyFrom, nil
}

func timeBlockOverlaps(startA, endA, startB, endB time.Time) bool {
	return startA.Before(endB) && endA.After(startB)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

