package handler

import (
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// textResponse wraps a plain text result into a standard MCP tool response
func textResponse(text string) *protocol.CallToolResponse {
	return &protocol.CallToolResponse{
		Content: []protocol.ToolContent{
			{
				Type: "text",
				Text: text,
			},
		},
	}
}
