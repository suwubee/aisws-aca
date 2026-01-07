package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	WorkflowTemplateCategoryDevelopment   = "development"
	WorkflowTemplateCategoryDevOps        = "devops"
	WorkflowTemplateCategoryDocumentation = "documentation"
	WorkflowTemplateCategoryTesting       = "testing"
)

// WorkflowTemplate represents a reusable workflow blueprint.
type WorkflowTemplate struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	Category    string    `gorm:"not null;index" json:"category"`
	Nodes       string    `gorm:"type:text" json:"nodes"` // JSON
	Edges       string    `gorm:"type:text" json:"edges"` // JSON
	IsBuiltin   bool      `gorm:"default:false;index" json:"is_builtin"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	builtinWorkflowTemplatePRD    = "workflow-template-prd"
	builtinWorkflowTemplateDevOps = "workflow-template-devops"
	builtinWorkflowTemplateBugFix = "workflow-template-bugfix"
)

func builtinWorkflowTemplates() []WorkflowTemplate {
	return []WorkflowTemplate{
		{
			ID:          builtinWorkflowTemplatePRD,
			Name:        "PRD编写流程",
			Description: "需求分析→PRD撰写→评审",
			Category:    WorkflowTemplateCategoryDocumentation,
			Nodes: `[
  {"id":"prd-1","type":"task","name":"需求分析","data":{"label":"需求分析"},"config":{"title":"需求分析","initial_prompt":"梳理业务背景、目标用户、核心问题与范围，列出关键需求点与优先级。"}},
  {"id":"prd-2","type":"task","name":"PRD撰写","data":{"label":"PRD撰写"},"config":{"title":"PRD撰写","initial_prompt":"根据需求分析输出PRD，包含目标、范围、功能点、交互说明、验收标准与风险。"}},
  {"id":"prd-3","type":"task","name":"评审","data":{"label":"评审"},"config":{"title":"评审","initial_prompt":"组织评审并收集反馈，明确修改项与结论，更新PRD并同步干系人。"}}
]`,
			Edges: `[
  {"id":"e-prd-1-2","source":"prd-1","target":"prd-2"},
  {"id":"e-prd-2-3","source":"prd-2","target":"prd-3"}
]`,
			IsBuiltin: true,
		},
		{
			ID:          builtinWorkflowTemplateDevOps,
			Name:        "DevOps全流程",
			Description: "代码检查→构建→测试→部署",
			Category:    WorkflowTemplateCategoryDevOps,
			Nodes: `[
  {"id":"devops-1","type":"task","name":"代码检查","data":{"label":"代码检查"},"config":{"title":"代码检查","initial_prompt":"运行lint/静态检查与格式化，修复阻塞问题并更新变更说明。"}},
  {"id":"devops-2","type":"task","name":"构建","data":{"label":"构建"},"config":{"title":"构建","initial_prompt":"执行构建流程，确保产物可生成且版本号/依赖正确。"}},
  {"id":"devops-3","type":"task","name":"测试","data":{"label":"测试"},"config":{"title":"测试","initial_prompt":"运行单元/集成测试并查看覆盖率，修复失败用例与关键回归。"}},
  {"id":"devops-4","type":"task","name":"部署","data":{"label":"部署"},"config":{"title":"部署","initial_prompt":"执行部署到目标环境，做健康检查、回滚预案确认与发布记录。"}}
]`,
			Edges: `[
  {"id":"e-devops-1-2","source":"devops-1","target":"devops-2"},
  {"id":"e-devops-2-3","source":"devops-2","target":"devops-3"},
  {"id":"e-devops-3-4","source":"devops-3","target":"devops-4"}
]`,
			IsBuiltin: true,
		},
		{
			ID:          builtinWorkflowTemplateBugFix,
			Name:        "Bug修复流程",
			Description: "问题分析→修复→测试→复盘",
			Category:    WorkflowTemplateCategoryDevelopment,
			Nodes: `[
  {"id":"bugfix-1","type":"task","name":"问题分析","data":{"label":"问题分析"},"config":{"title":"问题分析","initial_prompt":"复现问题、收集日志与上下文，定位根因与影响范围，提出修复方案。"}},
  {"id":"bugfix-2","type":"task","name":"修复","data":{"label":"修复"},"config":{"title":"修复","initial_prompt":"实现修复并补充必要的防御性检查，更新相关文档/注释。"}},
  {"id":"bugfix-3","type":"task","name":"测试","data":{"label":"测试"},"config":{"title":"测试","initial_prompt":"补充测试用例并回归验证，确保相关场景与边界条件覆盖。"}},
  {"id":"bugfix-4","type":"task","name":"复盘","data":{"label":"复盘"},"config":{"title":"复盘","initial_prompt":"总结问题原因、修复过程与改进措施，形成可执行的预防项。"}}
]`,
			Edges: `[
  {"id":"e-bugfix-1-2","source":"bugfix-1","target":"bugfix-2"},
  {"id":"e-bugfix-2-3","source":"bugfix-2","target":"bugfix-3"},
  {"id":"e-bugfix-3-4","source":"bugfix-3","target":"bugfix-4"}
]`,
			IsBuiltin: true,
		},
	}
}

func ensureBuiltinWorkflowTemplates(db *gorm.DB) error {
	if db == nil {
		return errors.New("missing database connection")
	}

	for _, tpl := range builtinWorkflowTemplates() {
		var existing WorkflowTemplate
		if err := db.First(&existing, "id = ?", tpl.ID).Error; err == nil {
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := db.Create(&tpl).Error; err != nil {
			return err
		}
	}

	return nil
}
