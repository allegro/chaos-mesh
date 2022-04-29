// Copyright 2021 Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package ipset

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/chaos-mesh/chaos-mesh/controllers/podnetworkchaos/netutils"
	chaosdaemonclient "github.com/chaos-mesh/chaos-mesh/pkg/chaosdaemon/client"
	pb "github.com/chaos-mesh/chaos-mesh/pkg/chaosdaemon/pb"
)

var log = ctrl.Log.WithName("ipset")

type IPSetType string

const (
	SetIPSet     IPSetType = "set"
	NetPortIPSet IPSetType = "netPort"
	NetIPSet     IPSetType = "net"
)

// BuildIPSet builds an ipset with provided pod ip list
func BuildIPSet(pods []v1.Pod, externalCidrs []v1alpha1.CidrAndPort, networkchaos *v1alpha1.NetworkChaos, namePostFix string, source string) v1alpha1.RawIPSet {
	setName := GenerateIPSetName(networkchaos, SetIPSet, namePostFix)
	netPortName := GenerateIPSetName(networkchaos, NetPortIPSet, namePostFix)
	netName := GenerateIPSetName(networkchaos, NetIPSet, namePostFix)
	cidrs := []v1alpha1.CidrAndPort{}

	for _, cidr := range externalCidrs {
		cidrs = append(cidrs, cidr)
	}

	for _, pod := range pods {
		if len(pod.Status.PodIP) > 0 {
			cidr := v1alpha1.CidrAndPort{Cidr: netutils.IPToCidr(pod.Status.PodIP)}
			cidrs = append(cidrs, cidr)
		}
	}

	return v1alpha1.RawIPSet{
		SetName:     setName,
		NetPortName: netPortName,
		NetName:     netName,
		Cidrs:       cidrs,
		RawRuleSource: v1alpha1.RawRuleSource{
			Source: source,
		},
	}
}

// GenerateIPSetName generates name for ipset
func GenerateIPSetName(networkchaos *v1alpha1.NetworkChaos, typ IPSetType, namePostFix string) string {
	return netutils.CompressName(networkchaos.Name, 27, fmt.Sprintf("%s_%s", typ, namePostFix))
}

// FlushIPSets makes grpc calls to chaosdaemon to save ipset
func FlushIPSets(ctx context.Context, pbClient chaosdaemonclient.ChaosDaemonClientInterface, pod *v1.Pod, ipsets []*pb.IPSet) error {
	var err error

	if len(pod.Status.ContainerStatuses) == 0 {
		return fmt.Errorf("%s %s can't get the state of container", pod.Namespace, pod.Name)
	}

	log.Info("Flushing IP Sets....")
	for _, containerStatus := range pod.Status.ContainerStatuses {
		containerID := containerStatus.ContainerID
		log.Info("attempting to flush ip set", "containerID", containerID)

		_, err = pbClient.FlushIPSets(ctx, &pb.IPSetsRequest{
			Ipsets:      ipsets,
			ContainerId: containerID,
			EnterNS:     true,
		})

		if err != nil {
			log.Error(err, fmt.Sprintf("error while flushing ip sets for containerID %s", containerID))
		} else {
			log.Info("Successfully flushed ip set")
			return nil
		}
	}

	return fmt.Errorf("unable to flush ip sets for pod %s", pod.Name)
}
