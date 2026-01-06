package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ruleSetImportEnvelope struct {
	Items    []model.RuleSet `json:"items"`
	RuleSets []model.RuleSet `json:"rule_sets"`
}

func parseRuleSetsImportBody(body []byte) ([]model.RuleSet, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}

	var ruleSets []model.RuleSet
	if err := json.Unmarshal(body, &ruleSets); err == nil {
		return ruleSets, nil
	}

	var env ruleSetImportEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}

	if len(env.RuleSets) > 0 {
		return env.RuleSets, nil
	}

	return env.Items, nil
}

func normalizeRuleSetForImport(ruleSet model.RuleSet) (model.RuleSet, error) {
	ruleSet.ID = strings.TrimSpace(ruleSet.ID)
	ruleSet.Name = strings.TrimSpace(ruleSet.Name)
	ruleSet.Type = strings.TrimSpace(ruleSet.Type)

	if ruleSet.ID == "" {
		return model.RuleSet{}, fmt.Errorf("rule set id is required")
	}
	if ruleSet.Type == "" {
		return model.RuleSet{}, fmt.Errorf("rule set type is required (id=%s)", ruleSet.ID)
	}

	switch ruleSet.Type {
	case "system", "task", "terminal":
	default:
		return model.RuleSet{}, fmt.Errorf("invalid rule set type %q (id=%s)", ruleSet.Type, ruleSet.ID)
	}

	return ruleSet, nil
}

func buildRuleSetsExportFilename(now time.Time) string {
	return fmt.Sprintf("rule_sets_%s.json", now.Format("20060102_150405"))
}

// ExportRuleSets 导出所有规则集为JSON文件
func (ctrl *AutomationController) ExportRuleSets(c *fiber.Ctx) error {
	var ruleSets []model.RuleSet
	if err := model.DB.Order("created_at asc").Find(&ruleSets).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to export rule sets"})
	}

	data, err := json.Marshal(ruleSets)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to build export file"})
	}

	filename := buildRuleSetsExportFilename(time.Now())
	c.Set("Content-Type", "application/json; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Set("Cache-Control", "no-store")
	return c.Send(data)
}

// ImportRuleSets 导入规则集JSON
func (ctrl *AutomationController) ImportRuleSets(c *fiber.Ctx) error {
	ruleSets, err := parseRuleSetsImportBody(c.Body())
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON file"})
	}
	if len(ruleSets) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "No rule sets found in import file"})
	}

	now := time.Now()
	normalized := make([]model.RuleSet, 0, len(ruleSets))
	for _, rs := range ruleSets {
		item, err := normalizeRuleSetForImport(rs)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		normalized = append(normalized, item)
	}

	var created, updated int
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, rs := range normalized {
			var existing model.RuleSet
			err := tx.First(&existing, "id = ?", rs.ID).Error
			if err == nil {
				rs.CreatedAt = existing.CreatedAt
				rs.UpdatedAt = now
				if err := tx.Save(&rs).Error; err != nil {
					return err
				}
				updated++
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			if rs.CreatedAt.IsZero() {
				rs.CreatedAt = now
			}
			rs.UpdatedAt = now
			if err := tx.Create(&rs).Error; err != nil {
				return err
			}
			created++
		}
		return nil
	}); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to import rule sets"})
	}

	ctrl.refreshAutomationForAllSessions()

	return c.JSON(fiber.Map{
		"message": "Rule sets imported",
		"created": created,
		"updated": updated,
		"total":   len(normalized),
	})
}
