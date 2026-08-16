package app

import (
	"fmt"

	"ejina-microgrid/internal/domain"
)

// CreateInspection schedules a new inspection task for the given entity.
func (s *Service) CreateInspection(entityType domain.EntityType, entityID, inspector string) (domain.Inspection, error) {
	now := s.now()
	ins := domain.Inspection{
		ID:         s.ids.Next("ins"),
		EntityType: entityType,
		EntityID:   entityID,
		Inspector:  inspector,
		Status:     domain.InspectionStatusPending,
		CreatedAt:  now,
	}
	if err := s.store.SaveInspection(ins); err != nil {
		return domain.Inspection{}, err
	}
	return ins, nil
}

// RecordInspectionResult completes an inspection. When the inspector reports an
// anomaly the method automatically creates a repair work order and links it
// back to the inspection, starting the repair closed-loop workflow.
func (s *Service) RecordInspectionResult(inspectionID, result string, anomaly bool) (domain.Inspection, *domain.WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ins, err := s.store.GetInspection(inspectionID)
	if err != nil {
		return domain.Inspection{}, nil, err
	}
	if ins.Status == domain.InspectionStatusCompleted {
		return domain.Inspection{}, nil, fmt.Errorf("inspection %s already completed", inspectionID)
	}

	now := s.now()
	ins.Status = domain.InspectionStatusCompleted
	ins.Result = result
	ins.Anomaly = anomaly
	ins.CompletedAt = now
	s.store.PutInspection(ins)

	if !anomaly {
		return ins, nil, nil
	}

	// Anomaly detected → create repair work order and dispatch immediately.
	wo := domain.WorkOrder{
		ID:          s.ids.Next("wo"),
		Title:       fmt.Sprintf("巡检异常- %s", ins.EntityID),
		EntityType:  ins.EntityType,
		EntityID:    ins.EntityID,
		Status:      domain.WorkOrderStatusDraft,
		Description: result,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.SaveWorkOrder(wo); err != nil {
		return ins, nil, err
	}
	ins.WorkOrderID = wo.ID
	s.store.PutInspection(ins)

	// Dispatch the work order to start the accept-timeout clock.
	wo, dispatchErr := s.dispatchWorkOrder(wo.ID)
	if dispatchErr != nil {
		return ins, nil, dispatchErr
	}
	return ins, &wo, nil
}

// ReportAnomaly allows operators to report an equipment anomaly directly (not
// via inspection). It evaluates the thermal/SOC rules for battery cabins and
// creates a repair work order.
func (s *Service) ReportAnomaly(entityType domain.EntityType, entityID, description string) (domain.WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()

	// For battery cabins, evaluate thermal and SOC rules.
	if entityType == domain.EntityTypeBatteryCabin {
		bc, err := s.store.GetBatteryCabin(entityID)
		if err != nil {
			return domain.WorkOrder{}, err
		}
		bc.EvaluateTemperature(s.cfg.BatteryTempThresholdC, now)
		s.store.PutBatteryCabin(bc)
	}

	wo := domain.WorkOrder{
		ID:          s.ids.Next("wo"),
		Title:       fmt.Sprintf("异常上报- %s", entityID),
		EntityType:  entityType,
		EntityID:    entityID,
		Status:      domain.WorkOrderStatusDraft,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.SaveWorkOrder(wo); err != nil {
		return domain.WorkOrder{}, err
	}
	return s.dispatchWorkOrder(wo.ID)
}
