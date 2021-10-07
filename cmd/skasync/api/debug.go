package api

import (
	"skasync/pkg/debug"
	"strconv"

	"github.com/labstack/echo/v4"
)

type DebugController struct {
	debugChangeList *debug.ChangeList
}

func NewDebugController(g *echo.Group, debugChangeList *debug.ChangeList) *DebugController {
	ctrl := &DebugController{debugChangeList}

	g.GET("/change-list/:id", ctrl.changeListHandler())

	return ctrl
}

func (ctrl *DebugController) changeListHandler() echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(200, echo.Map{
				"message": "incorrect id",
			})
		}

		list := ctrl.debugChangeList.Get(id)
		if list == nil {
			return c.JSON(200, echo.Map{
				"message": "not found",
			})
		}

		result := echo.Map{}

		for providerName, cl := range list {
			addedList := make([]string, 0, len(cl.Added()))
			for filePath := range cl.Added() {
				addedList = append(addedList, filePath)
			}

			modifiedList := make([]string, 0, len(cl.Modified()))
			for filePath := range cl.Modified() {
				modifiedList = append(modifiedList, filePath)
			}

			deletedList := make([]string, 0, len(cl.Deleted()))
			for filePath := range cl.Deleted() {
				deletedList = append(deletedList, filePath)
			}

			result[providerName] = echo.Map{
				"added":    addedList,
				"modified": modifiedList,
				"deleted":  deletedList,
			}
		}

		return c.JSON(200, result)
	}
}
