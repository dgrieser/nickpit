package llm

import "testing"

func TestRenderPromptIncrement(t *testing.T) {
	const prompt = `{{$item := 0}}{{$item = inc $item}}{{$item}}{{if .}} {{$item = inc $item}}{{$item}}{{end}} {{$item = inc $item}}{{$item}}`

	tests := []struct {
		name string
		data bool
		want string
	}{
		{name: "conditional included", data: true, want: "1 2 3"},
		{name: "conditional omitted", data: false, want: "1 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderPrompt(prompt, tt.data)
			if err != nil {
				t.Fatalf("RenderPrompt() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("RenderPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}
