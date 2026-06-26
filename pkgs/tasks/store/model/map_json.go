package model

import (
	"encoding/json"

	"gorm.io/datatypes"
)

func datatypesFromRaw(r json.RawMessage) datatypes.JSON {
	if len(r) == 0 {
		return datatypes.JSON("{}")
	}
	return datatypes.JSON(r)
}

func rawFromDatatypes(j datatypes.JSON) json.RawMessage {
	if len(j) == 0 {
		return nil
	}
	return json.RawMessage(j)
}

func jsonRawObject() json.RawMessage {
	return json.RawMessage("{}")
}

func rawJSONObjectFromDatatypes(j datatypes.JSON) json.RawMessage {
	if len(j) == 0 {
		return jsonRawObject()
	}
	return json.RawMessage(j)
}
