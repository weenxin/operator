package controller

import (
	"fmt"

	"github.com/appscode/go/log"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutil "kmodules.xyz/client-go"
	core_util "kmodules.xyz/client-go/core/v1"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/monitoring-agent-api/agents"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
)

func (c *Controller) newMonitorController(elasticsearch *api.Elasticsearch) (mona.Agent, error) {
	monitorSpec := elasticsearch.Spec.Monitor

	if monitorSpec == nil {
		return nil, fmt.Errorf("MonitorSpec not found in %v", elasticsearch.Spec)
	}

	if monitorSpec.Prometheus != nil {
		return agents.New(monitorSpec.Agent, c.Client, c.ApiExtKubeClient, c.promClient), nil
	}

	return nil, fmt.Errorf("monitoring controller not found for %v", monitorSpec)
}

func (c *Controller) addOrUpdateMonitor(elasticsearch *api.Elasticsearch) (kutil.VerbType, error) {
	agent, err := c.newMonitorController(elasticsearch)
	if err != nil {
		return kutil.VerbUnchanged, err
	}
	return agent.CreateOrUpdate(elasticsearch.StatsService(), elasticsearch.Spec.Monitor)
}

func (c *Controller) deleteMonitor(elasticsearch *api.Elasticsearch) (kutil.VerbType, error) {
	agent, err := c.newMonitorController(elasticsearch)
	if err != nil {
		return kutil.VerbUnchanged, err
	}
	return agent.Delete(elasticsearch.StatsService())
}

func (c *Controller) getOldAgent(elasticsearch *api.Elasticsearch) mona.Agent {
	service, err := c.Client.CoreV1().Services(elasticsearch.Namespace).Get(elasticsearch.StatsService().ServiceName(), metav1.GetOptions{})
	if err != nil {
		return nil
	}
	oldAgentType, _ := meta_util.GetStringValue(service.Annotations, mona.KeyAgent)
	return agents.New(mona.AgentType(oldAgentType), c.Client, c.ApiExtKubeClient, c.promClient)
}

func (c *Controller) setNewAgent(elasticsearch *api.Elasticsearch) error {
	service, err := c.Client.CoreV1().Services(elasticsearch.Namespace).Get(elasticsearch.StatsService().ServiceName(), metav1.GetOptions{})
	if err != nil {
		return err
	}
	_, _, err = core_util.PatchService(c.Client, service, func(in *core.Service) *core.Service {
		in.Annotations = core_util.UpsertMap(in.Annotations, map[string]string{
			mona.KeyAgent: string(elasticsearch.Spec.Monitor.Agent),
		},
		)
		return in
	})
	return err
}

func (c *Controller) manageMonitor(elasticsearch *api.Elasticsearch) error {
	oldAgent := c.getOldAgent(elasticsearch)
	if elasticsearch.Spec.Monitor != nil {
		if oldAgent != nil &&
			oldAgent.GetType() != elasticsearch.Spec.Monitor.Agent {
			if _, err := oldAgent.Delete(elasticsearch.StatsService()); err != nil {
				log.Errorf("error in deleting Prometheus agent. Reason: %v", err.Error())
			}
		}
		if _, err := c.addOrUpdateMonitor(elasticsearch); err != nil {
			return err
		}
		return c.setNewAgent(elasticsearch)
	} else if oldAgent != nil {
		if _, err := oldAgent.Delete(elasticsearch.StatsService()); err != nil {
			log.Errorf("error in deleting Prometheus agent. Reason: %v", err.Error())
		}
	}
	return nil
}
