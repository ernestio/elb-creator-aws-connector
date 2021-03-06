/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"encoding/json"
	"errors"
	"log"
)

var (
	ErrDatacenterIDInvalid          = errors.New("Datacenter VPC ID invalid")
	ErrDatacenterRegionInvalid      = errors.New("Datacenter Region invalid")
	ErrDatacenterCredentialsInvalid = errors.New("Datacenter credentials invalid")
	ErrELBNameInvalid               = errors.New("ELB name is invalid")
	ErrELBProtocolInvalid           = errors.New("ELB protocol invalid")
	ErrELBFromPortInvalid           = errors.New("ELB from port invalid")
	ErrELBToPortInvalid             = errors.New("ELB to port invalid")
)

type Listener struct {
	FromPort  int64  `json:"from_port"`
	ToPort    int64  `json:"to_port"`
	Protocol  string `json:"protocol"`
	SSLCertID string `json:"ssl_cert"`
}

// Event stores the elb data
type Event struct {
	UUID                string     `json:"_uuid"`
	BatchID             string     `json:"_batch_id"`
	ProviderType        string     `json:"_type"`
	DatacenterName      string     `json:"datacenter_name,omitempty"`
	DatacenterRegion    string     `json:"datacenter_region"`
	DatacenterToken     string     `json:"datacenter_token"`
	DatacenterSecret    string     `json:"datacenter_secret"`
	VPCID               string     `json:"vpc_id"`
	ELBName             string     `json:"name"`
	ELBIsPrivate        bool       `json:"is_private"`
	ELBListeners        []Listener `json:"listeners"`
	ELBDNSName          string     `json:"dns_name"`
	InstanceNames       []string   `json:"instance_names"`
	InstanceAWSIDs      []string   `json:"instance_aws_ids"`
	NetworkAWSIDs       []string   `json:"network_aws_ids"`
	SecurityGroups      []string   `json:"security_groups"`
	SecurityGroupAWSIDs []string   `json:"security_group_aws_ids"`
	ErrorMessage        string     `json:"error,omitempty"`
}

// Validate checks if all criteria are met
func (ev *Event) Validate() error {
	if ev.VPCID == "" {
		return ErrDatacenterIDInvalid
	}

	if ev.DatacenterRegion == "" {
		return ErrDatacenterRegionInvalid
	}

	if ev.DatacenterSecret == "" || ev.DatacenterToken == "" {
		return ErrDatacenterCredentialsInvalid
	}

	if ev.ELBName == "" {
		return ErrELBNameInvalid
	}

	// Validate Ports
	for _, listener := range ev.ELBListeners {
		if listener.Protocol == "" {
			return ErrELBProtocolInvalid
		}
		if listener.FromPort < 1 || listener.FromPort > 65535 {
			return ErrELBFromPortInvalid
		}
		if listener.ToPort < 1 || listener.ToPort > 65535 {
			return ErrELBToPortInvalid
		}

		if listener.Protocol != "HTTP" &&
			listener.Protocol != "HTTPS" &&
			listener.Protocol != "TCP" &&
			listener.Protocol != "SSL" {
			return ErrELBProtocolInvalid
		}
	}

	return nil
}

// Process the raw event
func (ev *Event) Process(data []byte) error {
	err := json.Unmarshal(data, &ev)
	if err != nil {
		nc.Publish("elb.create.aws.error", data)
	}
	return err
}

// Error the request
func (ev *Event) Error(err error) {
	log.Printf("Error: %s", err.Error())
	ev.ErrorMessage = err.Error()

	data, err := json.Marshal(ev)
	if err != nil {
		log.Panic(err)
	}
	nc.Publish("elb.create.aws.error", data)
}

// Complete the request
func (ev *Event) Complete() {
	data, err := json.Marshal(ev)
	if err != nil {
		ev.Error(err)
	}
	nc.Publish("elb.create.aws.done", data)
}
