package api

import (
	"skasync/pkg/k8s"
	"skasync/pkg/sync"

	"github.com/labstack/echo/v4"
)

type SyncController struct {
	podSyncer *sync.EndpointSyncker
	podsCtrl  *k8s.EndpointCtrl
}

func NewSyncController(g *echo.Group, podSyncer *sync.EndpointSyncker, podsCtrl *k8s.EndpointCtrl) *SyncController {
	ctrl := &SyncController{
		podSyncer: podSyncer,
		podsCtrl:  podsCtrl,
	}

	g.PUT("/in/pod", ctrl.syncInHandler())
	g.PUT("/in/allPods", ctrl.syncInToAllPodsHandler())

	return ctrl
}

func (ctrl *SyncController) syncInHandler() echo.HandlerFunc {
	type data struct {
		PodTag string `json:"podTag"`
		Path   string `json:"path"`
	}
	return func(c echo.Context) error {
		reqData := data{}

		if err := c.Bind(&reqData); err != nil {
			return c.JSON(200, echo.Map{
				"error": "incorect params",
			})
		}

		pod, err := ctrl.podsCtrl.FindByTag(reqData.PodTag)
		if err != nil {
			return c.JSON(200, echo.Map{
				"error": "pod not found",
			})
		}

		if err := ctrl.podSyncer.SyncLocalPathToPod(pod, reqData.Path); err != nil {
			return c.JSON(200, echo.Map{
				"error":   "sync error",
				"message": err.Error(),
			})
		}

		return c.JSON(200, echo.Map{
			"status": "OK",
		})
	}
}

func (ctrl *SyncController) syncInToAllPodsHandler() echo.HandlerFunc {
	type data struct {
		Path string `json:"path"`
	}
	return func(c echo.Context) error {
		reqData := data{}

		if err := c.Bind(&reqData); err != nil {
			return c.JSON(200, echo.Map{
				"error": "incorect params",
			})
		}

		if err := ctrl.podSyncer.SyncLocalPathToPods(reqData.Path); err != nil {
			return c.JSON(200, echo.Map{
				"error":   "sync error",
				"message": err.Error(),
			})
		}

		return c.JSON(200, echo.Map{
			"status": "OK",
		})
	}
}
