package session

import (
	"errors"
	"math"
	"time"

	"github.com/gorilla/sessions"
)

type Option func(*store) error

// WithMaxLength sets the maximum length of the session value.
// The default is 4096 bytes.
func WithMaxLength(maxLength int) Option {
	return func(s *store) error {
		if maxLength < 0 || maxLength > 10<<20 {
			return errors.New("invalid MaxLength, must be between 0 and 10MB")
		}
		s.maxLength = maxLength
		return nil
	}
}

// WithKeyPrefix sets the key prefix for the session store.
// The default is "session".
func WithKeyPrefix(keyPrefix string) Option {
	return func(s *store) error {
		s.keyPrefix = keyPrefix + "_"
		return nil
	}
}

// WithSerializer sets the serializer for the session store.
// The default is JSONSerializer.
func WithSerializer(serializer Serializer) Option {
	return func(s *store) error {
		s.serializer = serializer
		return nil
	}
}

// WithSessionOptions sets the session options for the session store.
// The default is a sessions.Options struct with Path set to "/".
func WithSessionOptions(so *sessions.Options) Option {
	return func(s *store) error {
		if so == nil {
			return errors.New("sessions.Options is nil")
		}
		s.options = so
		return nil
	}
}

// WithDefaultMaxAge sets the default MaxAge for the session store.
// The default is 5 minutes.
func WithDefaultMaxAge(defaultMaxAge time.Duration) Option {
	return func(s *store) error {
		if defaultMaxAge < 60*time.Minute {
			return errors.New("invalid DefaultMaxAge, must be at least 1 minute")
		}
		s.defaultMaxAge = int(math.Ceil(defaultMaxAge.Seconds()))
		return nil
	}
}
