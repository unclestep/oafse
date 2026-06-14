package port

import (
	"oafse/internal/domain/model"
)

type Extractor interface {
	Extract(fetchData *FetchData) (*model.Page, error)
}
