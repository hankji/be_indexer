package be_indexer

import (
	"bytes"
	"encoding/gob"
	"errors"
)

// indexBaseExported is a helper structure used to serialize indexBase data.
type indexBaseExported struct {
	FieldsData      map[BEField]*FieldDesc
	WildcardEntries Entries
	KSizeContainers []*EntriesContainer
}

func init() {
	gob.Register(indexBase{})
	gob.Register(&CompactBEIndex{})
	gob.Register(&KGroupsBEIndex{})
	gob.Register(&EntriesContainer{})
	gob.Register(FieldDesc{})
	gob.Register(Entries{})
	gob.Register(&DefaultEntriesHolder{})
}

func (bi *KGroupsBEIndex) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(indexBaseExported{
		FieldsData:      bi.fieldsData,
		WildcardEntries: bi.wildcardEntries,
		KSizeContainers: bi.kSizeContainers,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (bi *KGroupsBEIndex) GobDecode(data []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	temp := indexBaseExported{}
	if err := dec.Decode(&temp); err != nil {
		return err
	}
	bi.fieldsData = temp.FieldsData
	bi.wildcardEntries = temp.WildcardEntries
	bi.kSizeContainers = temp.KSizeContainers
	return nil
}

func (bi *indexBase) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(struct {
		FieldsData      map[BEField]*FieldDesc
		WildcardEntries Entries
	}{FieldsData: bi.fieldsData, WildcardEntries: bi.wildcardEntries})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (bi *indexBase) GobDecode(data []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	temp := struct {
		FieldsData      map[BEField]*FieldDesc
		WildcardEntries Entries
	}{}
	if err := dec.Decode(&temp); err != nil {
		return err
	}
	bi.fieldsData = temp.FieldsData
	bi.wildcardEntries = temp.WildcardEntries
	return nil
}

func (ec *EntriesContainer) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(struct {
		DefaultHolder EntriesHolder
		FieldHolder   map[BEField]EntriesHolder
	}{DefaultHolder: ec.defaultHolder, FieldHolder: ec.fieldHolder})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (ec *EntriesContainer) GobDecode(data []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	temp := struct {
		DefaultHolder EntriesHolder
		FieldHolder   map[BEField]EntriesHolder
	}{}
	if err := dec.Decode(&temp); err != nil {
		return err
	}
	ec.defaultHolder = temp.DefaultHolder
	ec.fieldHolder = temp.FieldHolder
	return nil
}

func (d *DefaultEntriesHolder) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(struct {
		Debug     bool
		MaxLen    int64
		AvgLen    int64
		PlEntries map[Term]Entries
	}{Debug: d.debug, MaxLen: d.maxLen, AvgLen: d.avgLen, PlEntries: d.plEntries})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (d *DefaultEntriesHolder) GobDecode(data []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	temp := struct {
		Debug     bool
		MaxLen    int64
		AvgLen    int64
		PlEntries map[Term]Entries
	}{}
	if err := dec.Decode(&temp); err != nil {
		return err
	}
	d.debug = temp.Debug
	d.maxLen = temp.MaxLen
	d.avgLen = temp.AvgLen
	d.plEntries = temp.PlEntries
	// Tokenizer configuration is runtime configuration and must be re-registered
	// after decoding. Older gob payload fields for Parser/FieldParser are ignored.
	d.fieldParser = nil
	return nil
}

func (c *CompactBEIndex) GobEncode() ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (c *CompactBEIndex) GobDecode(data []byte) error {
	return errors.New("not implemented")
}
