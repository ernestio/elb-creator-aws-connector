/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
	ecc "github.com/ernestio/ernest-config-client"
	"github.com/nats-io/nats"
)

var nc *nats.Conn
var natsErr error

func eventHandler(m *nats.Msg) {
	var e Event

	err := e.Process(m.Data)
	if err != nil {
		return
	}

	if err = e.Validate(); err != nil {
		e.Error(err)
		return
	}

	err = createELB(&e)
	if err != nil {
		e.Error(err)
		return
	}

	e.Complete()
}

func mapListeners(ev *Event) []*elb.Listener {
	var l []*elb.Listener

	for _, port := range ev.ELBPorts {
		l = append(l, &elb.Listener{
			Protocol:         aws.String(port.Protocol),
			LoadBalancerPort: aws.Int64(port.FromPort),
			InstancePort:     aws.Int64(port.ToPort),
			InstanceProtocol: aws.String(port.Protocol),
			SSLCertificateId: aws.String(port.SSLCertID),
		})
	}

	return l
}

func createELB(ev *Event) error {
	creds := credentials.NewStaticCredentials(ev.DatacenterAccessKey, ev.DatacenterAccessToken, "")
	svc := elb.New(session.New(), &aws.Config{
		Region:      aws.String(ev.DatacenterRegion),
		Credentials: creds,
	})

	// Create Loadbalancer
	req := elb.CreateLoadBalancerInput{
		LoadBalancerName: aws.String(ev.ELBName),
		Listeners:        mapListeners(ev),
	}

	if ev.ELBIsPrivate {
		req.Scheme = aws.String("internal")
	}

	for _, sg := range ev.SecurityGroupAWSIDs {
		req.SecurityGroups = append(req.SecurityGroups, aws.String(sg))
	}

	for _, subnet := range ev.NetworkAWSIDs {
		req.Subnets = append(req.Subnets, aws.String(subnet))
	}

	resp, err := svc.CreateLoadBalancer(&req)
	if err != nil {
		return err
	}

	if resp.DNSName != nil {
		ev.ELBDNSName = *resp.DNSName
	}

	// Add instances
	ireq := elb.RegisterInstancesWithLoadBalancerInput{
		LoadBalancerName: aws.String(ev.ELBName),
	}

	for _, instance := range ev.InstanceAWSIDs {
		ireq.Instances = append(ireq.Instances, &elb.Instance{
			InstanceId: aws.String(instance),
		})
	}

	_, err = svc.RegisterInstancesWithLoadBalancer(&ireq)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	nc = ecc.NewConfig(os.Getenv("NATS_URI")).Nats()

	fmt.Println("listening for elb.create.aws")
	nc.Subscribe("elb.create.aws", eventHandler)

	runtime.Goexit()
}
