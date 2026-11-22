package mks

import "fmt"

// hashFields replaces the plain value of Config.Hashed fields with the application's hash,
// right before the write. Values that are not strings, or are empty, are left alone: a
// partial update can leave a password out.
func (p *Panel) hashFields(c Config, data map[string]any) error {
	for _, field := range c.Hashed {
		raw, ok := data[field]
		if !ok {
			continue
		}
		plain, ok := raw.(string)
		if !ok || plain == "" {
			continue
		}
		hashed, err := p.opts.Auth.Hash.Make(plain)
		if err != nil {
			return fmt.Errorf("mks: hash field %q: %w", field, err)
		}
		data[field] = hashed
	}
	return nil
}
