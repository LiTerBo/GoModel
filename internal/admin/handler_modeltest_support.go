package admin

import "fmt"

func errInvalidProbe(name string) error {
	return fmt.Errorf("unknown probe %q (valid: chat, embeddings, function_calling)", name)
}
