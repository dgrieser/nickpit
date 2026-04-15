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
          "title": {"type": "string"},
          "body": {"type": "string"},
          "confidence_score": {"type": "number", "minimum": 0, "maximum": 1},
          "priority": {"type": "integer", "minimum": 0, "maximum": 3},
          "code_location": {
            "type": "object",
            "properties": {
              "absolute_file_path": {"type": "string"},
              "line_range": {
                "type": "object",
                "properties": {
                  "start": {"type": "integer"},
                  "end": {"type": "integer"}
                },
                "required": ["start","end"]
              }
            },
            "required": ["absolute_file_path","line_range"]
          }
        },
        "required": ["title","body","confidence_score","code_location"]
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
    "overall_correctness": {"type": "string", "enum": ["patch is correct","patch is incorrect"]},
    "overall_explanation": {"type": "string"},
    "overall_confidence_score": {"type": "number", "minimum": 0, "maximum": 1}
  },
  "required": ["findings","overall_correctness","overall_explanation","overall_confidence_score"]
}`)
