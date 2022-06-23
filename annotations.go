package panurge

import (
	"context"
	"net/http"
	"sync"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/google/uuid"
)

var annotationsKey struct{}

// AnnotationMiddleware adds annotation support to the request
// context.
func AnnotationMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := ContextWithAnnotations(r.Context())

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ContextWithAnnotations allows us to annotate the request context
// independently of the XRay instrumentation.
func ContextWithAnnotations(ctx context.Context) context.Context {
	seg := xray.GetSegment(ctx)

	annotations := ContextAnnotations{
		standalone: seg == nil || seg.Dummy || xray.SdkDisabled(),
		segment:    seg,
	}

	if annotations.standalone {
		annotations.id = uuid.New().String()
		annotations.annotations = make(map[string]interface{})
		annotations.metadata = make(map[string]interface{})
	}

	return context.WithValue(ctx, &annotationsKey, &annotations)
}

func AddUserAnnotation(ctx context.Context, user string) {
	ann, ok := ctx.Value(&annotationsKey).(*ContextAnnotations)
	if !ok {
		return
	}

	ann.SetUser(user)
}

func AddAnnotation[T AllowedAnnotationTypes](ctx context.Context, key string, value T) {
	ann, ok := ctx.Value(&annotationsKey).(*ContextAnnotations)
	if !ok {
		return
	}

	ann.AddAnnotation(key, value)
}

func AddMetadata(ctx context.Context, key string, value interface{}) {
	ann, ok := ctx.Value(&annotationsKey).(*ContextAnnotations)
	if !ok {
		return
	}

	ann.AddMetadata(key, value)
}

func GetContextAnnotations(ctx context.Context) *ContextAnnotations {
	if ctx == nil {
		return nil
	}

	ann, _ := ctx.Value(&annotationsKey).(*ContextAnnotations)

	return ann
}

type ContextAnnotations struct {
	standalone bool
	segment    *xray.Segment

	id   string
	user string

	m           sync.Mutex
	annotations map[string]interface{}
	metadata    map[string]interface{}
}

func (a *ContextAnnotations) AddAnnotation(key string, value interface{}) {
	if !a.standalone {
		_ = a.segment.AddAnnotation(key, value)
		return
	}

	a.m.Lock()
	defer a.m.Unlock()

	switch value.(type) {
	case bool, int, uint, float32, float64, string:
		a.annotations[key] = value
	}
}

func (a *ContextAnnotations) AddMetadata(key string, value interface{}) {
	if !a.standalone {
		_ = a.segment.AddMetadata(key, value)
		return
	}

	a.m.Lock()
	defer a.m.Unlock()

	a.metadata[key] = value
}

func (a *ContextAnnotations) GetID() string {
	if !a.standalone {
		a.segment.Lock()
		defer a.segment.Unlock()

		return a.segment.TraceID
	}

	a.m.Lock()
	defer a.m.Unlock()

	return a.id
}

func (a *ContextAnnotations) SetUser(user string) {
	if !a.standalone {
		a.segment.Lock()
		defer a.segment.Unlock()

		a.segment.User = user
	}

	a.m.Lock()
	defer a.m.Unlock()

	a.user = user
}

func (a *ContextAnnotations) GetUser() string {
	if !a.standalone {
		a.segment.Lock()
		defer a.segment.Unlock()

		return a.segment.User
	}

	a.m.Lock()
	defer a.m.Unlock()

	return a.user
}

func (a *ContextAnnotations) GetAnnotations() map[string]interface{} {
	if !a.standalone {
		a.segment.Lock()
		defer a.segment.Unlock()

		return copyInterfaceMap(a.segment.Annotations)
	}

	a.m.Lock()
	defer a.m.Unlock()

	return copyInterfaceMap(a.annotations)
}

func (a *ContextAnnotations) GetMetadata() map[string]interface{} {
	if !a.standalone {
		a.segment.Lock()
		defer a.segment.Unlock()

		if a.segment.Metadata == nil {
			return nil
		}

		return copyInterfaceMap(a.segment.Metadata["default"])
	}

	a.m.Lock()
	defer a.m.Unlock()

	return copyInterfaceMap(a.metadata)
}

func copyInterfaceMap(source map[string]interface{}) map[string]interface{} {
	if len(source) == 0 {
		return nil
	}

	annotations := make(map[string]interface{})

	for k, v := range source {
		annotations[k] = v
	}

	return annotations
}

type AllowedAnnotationTypes interface {
	bool | int | uint | float32 | float64 | string
}
