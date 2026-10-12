package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// GetPutawayReports GET /warehouse/putaway/reports  (damaged / short reports raised by putters)
func GetPutawayReports(c *gin.Context) {
	wid := c.MustGet("warehouse_id").(uint)

	var reports []models.PutterPutawayReport
	database.DB.Where("warehouse_id = ?", wid).Order("created_at DESC").Limit(200).Find(&reports)

	taskIDs := make([]uint, 0, len(reports))
	staffIDs := make([]uint, 0, len(reports))
	for _, r := range reports {
		taskIDs = append(taskIDs, r.TaskID)
		staffIDs = append(staffIDs, r.StaffID)
	}

	taskProduct := map[uint]uint{}
	productName := map[uint]string{}
	staffName := map[uint]string{}

	if len(reports) > 0 {
		var tasks []models.PutawayTask
		database.DB.Where("id IN ?", taskIDs).Find(&tasks)
		prodIDs := make([]uint, 0, len(tasks))
		for _, t := range tasks {
			taskProduct[t.ID] = t.ProductID
			prodIDs = append(prodIDs, t.ProductID)
		}
		if len(prodIDs) > 0 {
			var prods []models.Product
			database.DB.Where("id IN ?", prodIDs).Find(&prods)
			for _, p := range prods {
				productName[p.ID] = p.Name
			}
		}
		var staff []models.WarehouseStaff
		database.DB.Unscoped().Where("id IN ?", staffIDs).Find(&staff)
		for _, s := range staff {
			staffName[s.ID] = s.Name
		}
	}

	items := make([]gin.H, 0, len(reports))
	for _, r := range reports {
		pid := taskProduct[r.TaskID]
		items = append(items, gin.H{
			"id":           r.ID,
			"task_id":      r.TaskID,
			"product_id":   pid,
			"product_name": productName[pid],
			"putter_id":    r.StaffID,
			"putter_name":  staffName[r.StaffID],
			"type":         r.Type,
			"quantity":     r.Quantity,
			"reason":       r.Reason,
			"created_at":   r.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"reports": items})
}
