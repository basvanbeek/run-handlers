// Copyright (c) Bas van Beek 2025.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package session

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"

	"github.com/gorilla/sessions"
)

type Serializer interface {
	Deserialize(d []byte, s *sessions.Session) error
	Serialize(s *sessions.Session) ([]byte, error)
}
type JSONSerializer struct{}

func (j JSONSerializer) Serialize(s *sessions.Session) ([]byte, error) {
	m := make(map[string]interface{}, len(s.Values))
	for k, v := range s.Values {
		ks, ok := k.(string)
		if !ok {
			err := fmt.Errorf("non-string key value %v is not permitted", k)
			logger.Error("JSON serialization error", err)
			return nil, err
		}
		m[ks] = v
	}
	return json.Marshal(m)
}

func (j JSONSerializer) Deserialize(d []byte, s *sessions.Session) error {
	m := make(map[string]interface{})
	err := json.Unmarshal(d, &m)
	if err != nil {
		logger.Error("JSON deserialization error", err)
		return err
	}
	for k, v := range m {
		s.Values[k] = v
	}
	return nil
}

type GobSerializer struct{}

func (s GobSerializer) Serialize(ss *sessions.Session) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(ss.Values)
	if err == nil {
		return buf.Bytes(), nil
	}
	return nil, err
}

func (s GobSerializer) Deserialize(d []byte, ss *sessions.Session) error {
	dec := gob.NewDecoder(bytes.NewBuffer(d))
	return dec.Decode(&ss.Values)
}
