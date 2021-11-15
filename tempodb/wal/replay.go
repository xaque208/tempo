package wal

import (
	"bytes"
	"io"
	"os"

	"github.com/grafana/tempo/tempodb/backend"
	"github.com/grafana/tempo/tempodb/encoding"
	"github.com/grafana/tempo/tempodb/encoding/common"
)

// ReplayWALAndGetRecords replays a WAL file that could contain either traces or searchdata
func ReplayWALAndGetRecords(file *os.File, v encoding.VersionedEncoding, enc backend.Encoding, handleObj func([]byte) error) ([]common.Record, error, error) {
	dataReader, err := v.NewDataReader(backend.NewContextReaderWithAllReader(file), enc)
	if err != nil {
		return nil, nil, err
	}

	var buffer []byte
	var records []common.Record
	var warning error
	var pageLen uint32
	var id []byte
	objectReader := v.NewObjectReaderWriter()
	currentOffset := uint64(0)
	for {
		buffer, pageLen, err = dataReader.NextPage(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			warning = err
			break
		}

		reader := bytes.NewReader(buffer)
		id, buffer, err = objectReader.UnmarshalObjectFromReader(reader)
		if err != nil {
			warning = err
			break
		}
		// wal should only ever have one object per page, test that here
		_, _, err = objectReader.UnmarshalObjectFromReader(reader)
		if err != io.EOF {
			warning = err
			break
		}

		// handleObj is primarily used by search replay to record search data in block header
		err = handleObj(buffer)
		if err != nil {
			warning = err
			break
		}

		// make a copy so we don't hold onto the iterator buffer
		recordID := append([]byte(nil), id...)
		records = append(records, common.Record{
			ID:     recordID,
			Start:  currentOffset,
			Length: pageLen,
		})
		currentOffset += uint64(pageLen)
	}

	common.SortRecords(records)

	return records, warning, nil
}
