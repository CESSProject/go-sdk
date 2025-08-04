package chain

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/centrifuge/go-substrate-rpc-client/v4/registry"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/parser"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/pkg/errors"
)

var (
	ErrTxEventNotFound = errors.New("tx event not found")
)

// ParseTxResult parses transaction events to find the target event and validate transaction status.
// Processes events, checks for extrinsic success/failure, and matches the specified event name.
// Parameters:
//
//	caller - Keyring pair of the transaction caller
//	events - List of events from the transaction block
//	eventName - Target event name to search for (e.g., "Balances.Transfer")
//
// Returns:
//
//	Target event if found
//	Error if parsing fails, event not found, or transaction failed
func (cli *Client) ParseTxResult(caller signature.KeyringPair, events []*parser.Event, eventName string) (*parser.Event, error) {
	var (
		event   *parser.Event
		capture bool
	)
	acc, err := types.NewAccountID(caller.PublicKey)
	if err != nil {
		return event, errors.Wrap(err, "parse tx events error")
	}
	for _, e := range events {
		switch e.Name {
		case eventName:
			event = e
		case "TransactionPayment.TransactionFeePaid", "EvmAccountMapping.TransactionFeePaid":
			feePaid := types.EventTransactionPaymentTransactionFeePaid{}
			if err := DecodeEvent(e, &feePaid); err != nil {
				return event, errors.Wrap(err, "parse tx events error")
			}
			if !feePaid.Who.Equal(acc) {
				event = nil
				continue
			}
			capture = true
		case "System.ExtrinsicSuccess":
			capture = false
			if event != nil {
				return event, nil
			}
		case "System.ExtrinsicFailed":
			if capture {
				var eerr error
				txFailed := types.EventSystemExtrinsicFailed{}
				if err := DecodeEvent(e, &txFailed); err != nil {
					jb, _ := json.Marshal(e.Fields)
					eerr = errors.New(fmt.Sprintf("extrinsic failed: native event: %s", string(jb)))
				} else {
					eerr = cli.ParseSystemEventError(txFailed.DispatchError.ModuleError)
				}
				return event, eerr
			}
		}
	}
	return event, errors.Wrap(ErrTxEventNotFound, "parse tx events error")
}

// DecodeEvent decodes a blockchain event into a user-provided struct.
// Handles reflection to map event fields to struct fields, includes error recovery for panics.
// Parameters:
//
//	event - Parsed event data from the blockchain
//	value - Pointer to a struct to decode the event into
//
// Returns:
//
//	Error if decoding fails (invalid input, type mismatch, etc.)
func DecodeEvent(event *parser.Event, value any) (err error) {
	defer func() {
		d := recover()
		if d != nil {
			err = fmt.Errorf("%v", d)
		}
	}()

	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.Wrap(errors.New("no pointer or nill value"), "decode event error")
	}
	rv = rv.Elem()
	rt := rv.Type()
	if rt.Kind() != reflect.Struct {
		return errors.Wrap(errors.New("no struct"), "decode event error")
	}
	if rt.Field(0).Type == reflect.TypeOf(types.Phase{}) {
		rv.Field(0).Set(reflect.ValueOf(*event.Phase))
	}
	return errors.Wrap(DecodeFields(rv, reflect.ValueOf(event.Fields)), "decode event error")
}

// DecodeFields recursively decodes event fields into a target struct/type.
// Handles structs, arrays, slices, maps, and primitive types with type conversion.
// Parameters:
//
//	target - Reflect value of the target to decode into
//	fv - Reflect value of the source field data
//
// Returns:
//
//	Error if type mismatch or decoding fails
func DecodeFields(target, fv reflect.Value) error {
	tt, ft := target.Type(), fv.Type()
	if ft.Kind() == reflect.Interface {
		fv = fv.Elem()
	}

	if !fv.IsValid() || !target.CanSet() {
		return nil
	}

	switch tt.Kind() {
	case reflect.Struct:
		source := fv.Interface()
		fields, ok := source.(registry.DecodedFields)
		if !ok {
			if fv.CanConvert(reflect.TypeOf(uint8(0))) {
				index := source.(uint8)
				tfv := target.Field(int(index))
				tfv.SetBool(true)
				return nil
			}
			for i := 0; i < target.NumField() && i < fv.NumField(); i++ {
				if err := DecodeFields(target.Field(i), fv.Field(i)); err != nil {
					return err
				}
			}
		} else {
			offset := 0
			if tt.Field(0).Type == reflect.TypeOf(types.Phase{}) {
				offset = 1
			}
			for i, field := range fields {
				fieldName := ConvertName(field.Name)
				tfv := target.FieldByName(fieldName)

				if !tfv.IsValid() && tt.NumField() > i+offset {
					tfv = target.Field(i + offset)
				}
				if subfs, ok := field.Value.(registry.DecodedFields); ok {
					if len(subfs) == 1 && tfv.Kind() != reflect.Struct {
						field = subfs[0] //Unpacking Data
					}
				}
				if err := DecodeFields(tfv, reflect.ValueOf(field.Value)); err != nil {
					return err
				}
			}
		}
	case reflect.Array, reflect.Slice:
		var tmp reflect.Value
		et := tt.Elem()
		if fv.Kind() == reflect.Array {
			tmp = reflect.New(reflect.ArrayOf(fv.Len(), et)).Elem()
		} else {
			tmp = reflect.MakeSlice(reflect.SliceOf(et), fv.Len(), fv.Len())
		}

		tet := tmp.Type()

		if !tet.ConvertibleTo(tt) {
			return fmt.Errorf("type %v and %v mismatch", tet, tt)
		}
		for i := 0; i < fv.Len(); i++ {
			err := DecodeFields(tmp.Index(i), fv.Index(i).Elem())
			if err != nil {
				return err
			}
		}
		target.Set(tmp.Convert(tt))
	case reflect.Map:
		if fv.Kind() != reflect.Map {
			return fmt.Errorf("type %v and %v mismatch", ft, tt)
		}
		ttk, tte := tt.Key(), tt.Elem()
		tmp := reflect.MakeMap(reflect.MapOf(ttk, tte))
		iter := fv.MapRange()
		for iter.Next() {
			key := iter.Key()
			val := iter.Value()

			tk := reflect.New(ttk).Elem()
			tv := reflect.New(tte).Elem()
			if err := DecodeFields(tk, key); err != nil {
				return err
			}
			if err := DecodeFields(tv, val); err != nil {
				return err
			}
			tmp.SetMapIndex(tk, tv)
		}
		target.Set(tmp)
	default:
		if ft != tt {
			if !ft.ConvertibleTo(tt) {
				return fmt.Errorf("type %v and %v mismatch", ft, tt)
			}
			target.Set(fv.Convert(tt))
		} else {
			target.Set(fv)
		}

	}
	return nil
}

// ConvertName converts snake_case field names to CamelCase for struct field mapping.
// Processes namespaces (e.g., "transaction_fee_paid" → "TransactionFeePaid").
// Parameters:
//
//	name - Original field name in snake_case format
//
// Returns:
//
//	CamelCase formatted name
func ConvertName(name string) string {
	var res string
	bases := strings.Split(name, ".")
	words := strings.Split(bases[len(bases)-1], "_")
	for _, word := range words {
		if word != "" {
			res += strings.ToUpper(string(word[0])) + word[1:]
		}
	}
	return res
}
