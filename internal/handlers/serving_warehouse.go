package handlers

import (
    "sort"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

type warehouseDist struct {
    wh   models.Warehouse
    dist float64
}

// FindServingWarehouse returns the closest active, non-mother warehouse whose
// service area actually covers (lat, lng): its polygon if it has one, else its
// radius. Returns (nil, 0, "", nil) when no warehouse serves the point (BUG-68).
func FindServingWarehouse(lat, lng float64) (*models.Warehouse, float64, string, error) {
    var warehouses []models.Warehouse
    if err := database.DB.Where("is_active = ? AND status = ? AND (warehouse_type IS NULL OR warehouse_type <> ?)", true, "open", "mother_warehouse").Find(&warehouses).Error; err != nil {
        return nil, 0, "", err
    }
    if len(warehouses) == 0 {
        return nil, 0, "", nil
    }
    cands := make([]warehouseDist, 0, len(warehouses))
    ids := make([]uint, 0, len(warehouses))
    for _, w := range warehouses {
        cands = append(cands, warehouseDist{wh: w, dist: haversineDistanceKm(lat, lng, w.Lat, w.Lng)})
        ids = append(ids, w.ID)
    }
    sort.Slice(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })

    type areaRow struct {
        ID            uint
        HasPolygon    bool
        ContainsPoint bool
    }
    var rows []areaRow
    if err := database.DB.Raw(
        `SELECT id, service_area IS NOT NULL AS has_polygon, CASE WHEN service_area IS NOT NULL THEN ST_Contains(service_area, ST_SetSRID(ST_MakePoint(?, ?), 4326)) ELSE false END AS contains_point FROM warehouses WHERE id IN ?`,
        lng, lat, ids,
    ).Scan(&rows).Error; err != nil {
        return nil, 0, "", err
    }
    byID := make(map[uint]areaRow, len(rows))
    for _, r := range rows {
        byID[r.ID] = r
    }

    for _, cd := range cands {
        a := byID[cd.wh.ID]
        if a.HasPolygon {
            if a.ContainsPoint {
                w := cd.wh
                return &w, cd.dist, "polygon", nil
            }
            continue
        }
        if cd.dist <= cd.wh.ServiceRadiusKm {
            w := cd.wh
            return &w, cd.dist, "radius", nil
        }
    }
    return nil, 0, "", nil
}