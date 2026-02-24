package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type EinoToolAdapter struct {
	tool Tool
}

func NewEinoToolAdapter(t Tool) *EinoToolAdapter {
	if t == nil {
		return nil
	}
	return &EinoToolAdapter{tool: t}
}

func (t *EinoToolAdapter) Info(ctx context.Context) (*schema.ToolInfo, error) {
	_ = ctx
	spec := t.tool.Spec()
	return toToolInfo(spec), nil
}

func (t *EinoToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
	_ = opts
	args := map[string]interface{}{}
	if strings.TrimSpace(argumentsInJSON) != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", err
		}
	}

	// 添加工具调用日志
	spec := t.tool.Spec()
	argsJSON, _ := json.MarshalIndent(args, "", "  ")
	println("\n================================================================================")
	printf("[TOOL CALL] 工具名称: %s\n", spec.Name)
	printf("[TOOL CALL] 工具描述: %s\n", spec.Description)
	printf("[TOOL CALL] 调用参数:\n%s\n", string(argsJSON))

	result, err := t.tool.Execute(ctx, args)

	// 添加工具执行结果日志（限制输出）
	if err != nil {
		printf("[TOOL ERROR] 执行错误: %v\n", err)
	}
	if result.Error != "" {
		printf("[TOOL ERROR] 结果错误: %s\n", result.Error)
	}

	// 智能输出结果摘要
	printToolResultSummary(result)

	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	payload, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return "", marshalErr
	}
	return string(payload), nil
}

// printToolResultSummary 打印工具结果的摘要，避免输出过多数据
func printToolResultSummary(result ToolResult) {
	if result.Output == nil {
		printf("[TOOL RESULT] 无输出数据\n")
		println("================================================================================")
		return
	}

	// 将输出转换为JSON以检查数据量
	outputJSON, _ := json.Marshal(result.Output)
	outputStr := string(outputJSON)

	// 如果数据量小于500字符，直接输出
	if len(outputStr) <= 500 {
		printf("[TOOL RESULT] 返回数据:\n%s\n", outputStr)
	} else {
		// 尝试识别数据类型并输出摘要
		switch v := result.Output.(type) {
		case []KlinePoint:
			printf("[TOOL RESULT] K线数据: 共 %d 条记录\n", len(v))
			if len(v) > 0 {
				printf("  时间范围: %s 至 %s\n", v[0].Time, v[len(v)-1].Time)
				printf("  前3条数据:\n")
				for i := 0; i < 3 && i < len(v); i++ {
					data, _ := json.Marshal(v[i])
					printf("    %s\n", string(data))
				}
				if len(v) > 3 {
					printf("  ... (省略 %d 条)\n", len(v)-3)
				}
			}
		case []IntradayPoint:
			printf("[TOOL RESULT] 分时数据: 共 %d 条记录\n", len(v))
			if len(v) > 0 {
				printf("  时间范围: %s 至 %s\n", v[0].Time, v[len(v)-1].Time)
				printf("  前3条数据:\n")
				for i := 0; i < 3 && i < len(v); i++ {
					data, _ := json.Marshal(v[i])
					printf("    %s\n", string(data))
				}
				if len(v) > 3 {
					printf("  ... (省略 %d 条)\n", len(v)-3)
				}
			}
		case []interface{}:
			printf("[TOOL RESULT] 列表数据: 共 %d 条记录\n", len(v))
			printf("  数据大小: %d 字节 (已省略详细输出)\n", len(outputStr))
		case map[string]interface{}:
			printf("[TOOL RESULT] 对象数据\n")
			// 输出键值摘要
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			printf("  包含字段: %v\n", keys)
			if len(outputStr) <= 300 {
				printf("  数据内容:\n%s\n", outputStr)
			} else {
				printf("  数据大小: %d 字节 (已省略详细输出)\n", len(outputStr))
			}
		default:
			printf("[TOOL RESULT] 数据类型: %T\n", v)
			printf("  数据大小: %d 字节 (已省略详细输出)\n", len(outputStr))
		}
	}
	println("================================================================================")
}

func BuildEinoTools(registry *Registry, allow []ToolSpec) []einotool.BaseTool {
	if registry == nil {
		return nil
	}
	allowed := map[string]ToolSpec{}
	for _, spec := range allow {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		allowed[spec.Name] = spec
	}
	tools := make([]einotool.BaseTool, 0)
	for _, t := range registry.ListTools() {
		if t == nil {
			continue
		}
		spec := t.Spec()
		if len(allowed) > 0 {
			if _, ok := allowed[spec.Name]; !ok {
				continue
			}
		}
		adapter := NewEinoToolAdapter(t)
		if adapter != nil {
			tools = append(tools, adapter)
		}
	}
	return tools
}

func toToolInfo(spec ToolSpec) *schema.ToolInfo {
	info := &schema.ToolInfo{
		Name: spec.Name,
		Desc: spec.Description,
	}
	if len(spec.Params) == 0 {
		return info
	}
	params := map[string]*schema.ParameterInfo{}
	for name, desc := range spec.Params {
		if strings.TrimSpace(name) == "" {
			continue
		}
		params[name] = &schema.ParameterInfo{
			Type:     schema.String,
			Desc:     desc,
			Required: contains(spec.Required, name),
		}
	}
	if len(params) > 0 {
		info.ParamsOneOf = schema.NewParamsOneOfByParams(params)
	}
	return info
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// 辅助函数
func printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}
