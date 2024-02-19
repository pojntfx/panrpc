package utils

type Request[T any] struct {
	Call     string `json:"call"`
	Function string `json:"function"`
	Args     []T    `json:"args"`
}

func (r *Request[T]) Marshal(marshal func(v any) (T, error)) (T, error) {
	return marshal(r)
}

func (r *Request[T]) Unmarshal(data T, unmarshal func(data T, v any) error) error {
	return unmarshal(data, r)
}

type Response[T any] struct {
	Call  string `json:"call"`
	Value T      `json:"value"`
	Err   string `json:"err"`
}

func (r *Response[T]) Marshal(marshal func(v any) (T, error)) (T, error) {
	return marshal(r)
}

func (r *Response[T]) Unmarshal(data T, unmarshal func(data T, v any) error) error {
	return unmarshal(data, r)
}
