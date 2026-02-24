package dictionary

import (
	"context"
	"fmt"

	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/dictionary"
	"github.com/leeforge/core/server/ent/dictionarydetail"
	"github.com/leeforge/framework/component"
)

// DictionaryComponent 字典组件实现
type DictionaryComponent struct {
	client *ent.Client
}

// NewDictionaryComponent 创建字典组件
func NewDictionaryComponent(client *ent.Client) *DictionaryComponent {
	return &DictionaryComponent{client: client}
}

// Name 返回组件名称
func (c *DictionaryComponent) Name() string {
	return "dictionary"
}

// Validate 验证字段值
func (c *DictionaryComponent) Validate(
	ctx context.Context,
	config map[string]any,
	value any,
) error {
	// 1. 解析配置
	cfg, err := c.parseConfig(config)
	if err != nil {
		return component.NewValidationError(c.Name(), "", err.Error(), err)
	}

	// 2. 类型检查
	strValue, ok := value.(string)
	if !ok {
		return component.NewValidationError(
			c.Name(),
			"",
			fmt.Sprintf("expected string, got %T", value),
			nil,
		)
	}

	// 3. 验证值是否存在
	exists, err := c.client.DictionaryDetail.Query().
		Where(
			dictionarydetail.HasDictionaryWith(dictionary.CodeEQ(cfg.Code)),
			dictionarydetail.ValueEQ(strValue),
			dictionarydetail.StatusEQ(true),
		).
		Exist(ctx)

	if err != nil {
		return component.NewValidationError(
			c.Name(),
			"",
			"validation query failed",
			err,
		)
	}

	if !exists {
		return component.NewValidationError(
			c.Name(),
			"",
			fmt.Sprintf("invalid value '%s' for dictionary '%s'", strValue, cfg.Code),
			nil,
		)
	}

	return nil
}

// GetOptions 获取可选项
func (c *DictionaryComponent) GetOptions(
	ctx context.Context,
	config map[string]any,
) ([]component.Option, error) {
	// 1. 解析配置
	cfg, err := c.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("parse config failed: %w", err)
	}

	// 2. 查询字典明细
	details, err := c.client.DictionaryDetail.Query().
		Where(
			dictionarydetail.HasDictionaryWith(dictionary.CodeEQ(cfg.Code)),
			dictionarydetail.StatusEQ(true),
		).
		Order(ent.Asc(dictionarydetail.FieldSort)).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("query options failed: %w", err)
	}

	// 3. 转换为 Option
	options := make([]component.Option, len(details))
	for i, detail := range details {
		extra := component.ParseExtendJSON(detail.Extend)
		options[i] = component.Option{
			Label: detail.Label,
			Value: detail.Value,
			Extra: extra,
		}
	}

	return options, nil
}

// PopulateDisplay 填充显示信息
func (c *DictionaryComponent) PopulateDisplay(
	ctx context.Context,
	config map[string]any,
	values []any,
) (map[any]component.Display, error) {
	// 0. 检查是否需要填充
	if !component.ShouldPopulateDisplay(config) {
		return nil, nil
	}

	// 1. 解析配置
	cfg, err := c.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("parse config failed: %w", err)
	}

	// 2. 转换 values 为字符串切片
	strValues := make([]string, 0, len(values))
	for _, v := range values {
		if strValue, ok := v.(string); ok {
			strValues = append(strValues, strValue)
		}
	}

	if len(strValues) == 0 {
		return make(map[any]component.Display), nil
	}

	// 3. 批量查询字典明细
	details, err := c.client.DictionaryDetail.Query().
		Where(
			dictionarydetail.HasDictionaryWith(dictionary.CodeEQ(cfg.Code)),
			dictionarydetail.ValueIn(strValues...),
		).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("query display failed: %w", err)
	}

	// 4. 获取 display 配置
	displayCfg := component.ExtractDisplayConfig(config)

	// 5. 构建映射
	displayMap := make(map[any]component.Display, len(details))
	for _, detail := range details {
		extra := component.ParseExtendJSON(detail.Extend)

		// 根据 display.fields 过滤字段
		filteredExtra := component.FilterFields(extra, displayCfg.Fields)

		displayMap[detail.Value] = component.Display{
			Label: detail.Label,
			Value: detail.Value,
			Extra: filteredExtra,
		}
	}

	return displayMap, nil
}

// DictionaryConfig 字典组件配置
type DictionaryConfig struct {
	Code string `json:"code"`
}

// parseConfig 解析配置
func (c *DictionaryComponent) parseConfig(config map[string]any) (*DictionaryConfig, error) {
	cfg := &DictionaryConfig{}

	if err := component.ParseConfig(config, cfg); err != nil {
		return nil, err
	}

	if cfg.Code == "" {
		return nil, fmt.Errorf("config.code is required")
	}

	return cfg, nil
}
