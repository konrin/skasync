package skaffold

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"skasync/pkg/k8s"
	"sync"
	"time"
)

type StatusProbe struct {
	listenAddr string

	podsCtrl *k8s.PodsCtrl
	eventsCh chan EventPkg

	mu          sync.Mutex
	subscribers []func(SkaffoldProcessStatus)
}

type SkaffoldProcessStatus struct {
	Deploy    string
	Artifacts map[string]string
	IsReady,
	DoesNotAnswer bool
}

type EventPkg struct {
	Result map[string]struct {
		Timestamp string                 `json:"timestamp"`
		Entry     string                 `json:"entry"`
		Event     map[string]interface{} `json:"event"`
	} `json:"result"`
}

func NewStatusProbe(listenAddr string, podsCtrl *k8s.PodsCtrl) *StatusProbe {
	return &StatusProbe{
		listenAddr:  listenAddr,
		podsCtrl:    podsCtrl,
		subscribers: make([]func(SkaffoldProcessStatus), 0),

		// TODO
		eventsCh: make(chan EventPkg, 10),
	}
}

// TODO
func (sp *StatusProbe) listenEvents(ctx context.Context) error {
	client := http.Client{
		Timeout: time.Duration(1<<63 - 1), // forever
	}

	resp, err := client.Get("http://" + sp.listenAddr + "/v1/events")
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		resp.Body.Close()
	}()

	bodyBuffer := make([]byte, 2048)

	for {
		n, err := resp.Body.Read(bodyBuffer)
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		eventPkgBytes := bodyBuffer[:n]

		pkg := EventPkg{}
		if err := json.Unmarshal(eventPkgBytes, &pkg); err != nil {
			println(err.Error())
			continue
		}

		sp.eventsCh <- pkg
	}
}

func (sp *StatusProbe) getState() (SkaffoldProcessStatus, error) {
	client := http.Client{}

	status := SkaffoldProcessStatus{}

	resp, err := client.Get("http://" + sp.listenAddr + "/v1/state")
	if err != nil {
		status.DoesNotAnswer = true
		return status, nil
	}

	if resp.StatusCode != http.StatusOK {
		status.IsReady = false
		return status, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return status, err
	}

	state := make(map[string]interface{})

	if err := json.Unmarshal(data, &state); err != nil {
		return status, err
	}

	deployStatus := ""
	statusCheck := ""
	buildArtifactsState := make(map[string]interface{})

	if state["deployState"] != nil {
		deployState := state["deployState"].(map[string]interface{})
		deployStatus = deployState["status"].(string)
	}

	if state["statusCheckState"] != nil {
		statusCheckState := state["statusCheckState"].(map[string]interface{})
		statusCheck = statusCheckState["status"].(string)
	}

	if state["buildState"] != nil {
		buildState := state["buildState"].(map[string]interface{})
		buildArtifactsState = buildState["artifacts"].(map[string]interface{})
	}

	artifacts := map[string]string{}

	isReady := true

	if deployStatus != "Complete" {
		isReady = false
	}

	if statusCheck != "Succeeded" {
		isReady = false
	}

	for _, pod := range sp.podsCtrl.GetPods() {
		artifactStatus, ok := buildArtifactsState[pod.Artifact]
		if !ok {
			// return SkaffoldProcessStatus{}, fmt.Errorf("image \"%s\" not found in configuration", pod.Artifact)
			artifacts[pod.Artifact] = "Not found"
			continue
		}

		status := artifactStatus.(string)
		artifacts[pod.Artifact] = status
	}

	status.Deploy = deployStatus
	status.Artifacts = artifacts
	status.IsReady = isReady

	return status, nil
}

func (sp *StatusProbe) Subscribe(fn func(SkaffoldProcessStatus)) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.subscribers = append(sp.subscribers, fn)
	return nil
}

func (sp *StatusProbe) Listen(ctx context.Context) error {
	ticker := time.NewTicker(time.Millisecond * 500)

	for {
		select {
		case <-ticker.C:
			state, err := sp.getState()
			if err != nil {
				return err
			}

			for _, sub := range sp.subscribers {
				sub(state)
			}
		case <-ctx.Done():
			ticker.Stop()
			return nil
		}
	}
}
