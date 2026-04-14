package llm

import "encoding/json"

var FindingsSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "severity": {"type": "string", "enum": ["info","warning","error","critical"]},
          "category": {"type": "string"},
          "file_path": {"type": "string"},
          "start_line": {"type": "integer"},
          "end_line": {"type": "integer"},
          "title": {"type": "string"},
          "description": {"type": "string"},
          "suggestion": {"type": "string"},
          "confidence": {"type": "number", "minimum": 0, "maximum": 1}
        },
        "required": ["severity","category","file_path","title","description","confidence"]
      }
    },
    "follow_up_requests": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "type": {"type": "string", "enum": ["file","lines","function","callers","callees"]},
          "path": {"type": "string"},
          "symbol": {"type": "string"},
          "start_line": {"type": "integer"},
          "end_line": {"type": "integer"},
          "depth": {"type": "integer"},
          "reason": {"type": "string"}
        },
        "required": ["type","reason"]
      }
    },
    "summary": {"type": "string"}
  },
  "required": ["findings","summary"]
}`)
