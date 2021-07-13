package migrations

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type storageFormatV5MigrationResult struct {
	key     ledger.Key
	payload *ledger.Payload
	err     error
}

type StorageFormatV5Migration struct {
	Log           zerolog.Logger
	OutputDir     string
	accounts      *state.Accounts
	programs      *programs.Programs
	brokenTypeIDs map[common.TypeID]brokenTypeCause
	reportFile    *os.File
}

type brokenTypeCause int

const (
	brokenTypeCauseParsingCheckingError brokenTypeCause = iota
	brokenTypeCauseMissingCompositeType
)

func (m StorageFormatV5Migration) filename() string {
	return path.Join(m.OutputDir, fmt.Sprintf("migration_report_%d.csv", int32(time.Now().Unix())))
}

func (m *StorageFormatV5Migration) Migrate(payloads []ledger.Payload) ([]ledger.Payload, error) {

	filename := m.filename()
	m.Log.Info().Msgf("Running storage format V5 migration. Saving report to %s.", filename)

	reportFile, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = reportFile.Close()
		if err != nil {
			panic(err)
		}
	}()

	m.reportFile = reportFile

	m.Log.Info().Msg("Loading account contracts ...")

	m.accounts = m.getContractsOnlyAccounts(payloads)

	m.Log.Info().Msg("Loaded account contracts")

	m.programs = programs.NewEmptyPrograms()
	m.brokenTypeIDs = make(map[common.TypeID]brokenTypeCause, 0)


	migratedPayloads := make([]ledger.Payload, 0, len(payloads))

	for _, payload := range payloads {

		keyParts := payload.Key.KeyParts

		rawOwner := keyParts[0].Value
		rawKey := keyParts[2].Value

		result := m.migrate(payload)

		if result.err != nil {

			return nil, fmt.Errorf(
				"failed to migrate key: %q (owner: %x): %w",
				rawKey,
				rawOwner,
				result.err,
			)
		} else if result.payload != nil {
			migratedPayloads = append(migratedPayloads, *result.payload)
		} else {
			m.Log.Warn().Msgf("DELETED key %q (owner: %x)", rawKey, rawOwner)
			m.reportFile.WriteString(fmt.Sprintf("%x,%s,DELETED\n", rawOwner, string(rawKey)))
		}
	}

	return migratedPayloads, nil
}

func (m StorageFormatV5Migration) getContractsOnlyAccounts(payloads []ledger.Payload) *state.Accounts {
	var filteredPayloads []ledger.Payload

	for _, payload := range payloads {
		rawKey := string(payload.Key.KeyParts[2].Value)
		if strings.HasPrefix(rawKey, "contract_names") ||
			strings.HasPrefix(rawKey, "code.") ||
			rawKey == "exists" {

			filteredPayloads = append(filteredPayloads, payload)
		}
	}

	l := newView(filteredPayloads)
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)
	return accounts
}

func (m StorageFormatV5Migration) migrate(
	payload ledger.Payload,
) storageFormatV5MigrationResult {
	migratedPayload, err := m.reencodePayload(payload)
	result := storageFormatV5MigrationResult{
		key: payload.Key,
	}
	if err != nil {
		result.err = err
	} else if migratedPayload != nil {
		if err := m.checkStorageFormat(*migratedPayload); err != nil {
			panic(fmt.Errorf("%w: key = %s", err, payload.Key.String()))
		}
		result.payload = migratedPayload
	}
	return result
}

func (m StorageFormatV5Migration) checkStorageFormat(payload ledger.Payload) error {

	if !bytes.HasPrefix(payload.Value, []byte{0x0, 0xca, 0xde}) {
		return nil
	}

	_, version := interpreter.StripMagic(payload.Value)
	if version != interpreter.CurrentEncodingVersion {
		return fmt.Errorf("invalid version for key %s: %d", payload.Key.String(), version)
	}

	return nil
}

var storageMigrationV5DecMode = func() cbor.DecMode {
	decMode, err := cbor.DecOptions{
		IntDec:           cbor.IntDecConvertNone,
		MaxArrayElements: math.MaxInt32,
		MaxMapPairs:      math.MaxInt32,
		MaxNestedLevels:  256,
	}.DecMode()
	if err != nil {
		panic(err)
	}
	return decMode
}()

func (m StorageFormatV5Migration) reencodePayload(
	payload ledger.Payload,
) (*ledger.Payload, error) {

	keyParts := payload.Key.KeyParts

	rawOwner := keyParts[0].Value
	rawController := keyParts[1].Value
	rawKey := keyParts[2].Value

	// Ignore known payload keys that are not Cadence values

	if state.IsFVMStateKey(
		string(rawOwner),
		string(rawController),
		string(rawKey),
	) {
		return &payload, nil
	}

	value, version := interpreter.StripMagic(payload.Value)

	if version != interpreter.CurrentEncodingVersion-1 {
		return nil,
			fmt.Errorf(
				"invalid storage format version for key: %s: %d",
				rawKey,
				version,
			)
	}

	err := storageMigrationV5DecMode.Valid(value)
	if err != nil {
		return &payload, nil
	}

	// Delete known dead or orphaned contract value child keys

	if m.isOrphanContactValueChildKey(
		rawKey,
		flow.BytesToAddress(rawOwner),
	) {
		m.Log.Warn().Msgf(
			"DELETING orphaned contract value child key: %s (owner: %x)",
			string(rawKey), rawOwner,
		)

		return nil, nil
	}

	// Extract the owner from the key

	newValue, keep, err := m.reencodeValue(
		value,
		common.BytesToAddress(rawOwner),
		string(rawKey),
		version,
	)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to re-encode key: %s: %w\n\nvalue:\n%s\n\n%s",
				rawKey, err,
				hex.Dump(value),
				cborMeLink(value),
			)
	}
	if !keep {
		return nil, nil
	}

	payload.Value = interpreter.PrependMagic(
		newValue,
		interpreter.CurrentEncodingVersion,
	)

	return &payload, nil
}

func cborMeLink(value []byte) string {
	return fmt.Sprintf("http://cbor.me/?bytes=%x", value)
}

const cborTagStorageReference = 202

var storageReferenceEncodingStart = []byte{0xd8, cborTagStorageReference}
var emptyArrayEncoding = []byte{0x80}

func (m StorageFormatV5Migration) reencodeValue(
	data []byte,
	owner common.Address,
	key string,
	version uint16,
) (
	newData []byte,
	keep bool,
	err error,
) {

	if bytes.Compare(data, emptyArrayEncoding) == 0 {
		m.Log.Warn().
			Str("key", key).
			Str("owner", owner.String()).
			Msgf("DELETING empty array")

		return nil, false, nil
	}

	// Decode the value

	path := []string{key}

	rootValue, err := interpreter.DecodeValueV4(data, &owner, path, version, nil)
	if err != nil {
		if tagErr, ok := err.(interpreter.UnsupportedTagDecodingError); ok &&
			tagErr.Tag == cborTagStorageReference &&
			bytes.Compare(data[:2], storageReferenceEncodingStart) == 0 {

			m.Log.Warn().
				Str("key", key).
				Str("owner", owner.String()).
				Msgf("DELETING unsupported storage reference")

			return nil, false, nil

		} else {
			return nil, false, fmt.Errorf(
				"failed to decode value: %w\n\nvalue:\n%s\n",
				err, hex.Dump(data),
			)
		}
	}

	// Force decoding of all container values

	interpreter.InspectValue(
		rootValue,
		func(inspectedValue interpreter.Value) bool {
			switch inspectedValue := inspectedValue.(type) {
			case *interpreter.CompositeValue:
				_ = inspectedValue.Fields()
			case *interpreter.ArrayValue:
				_ = inspectedValue.Elements()
			case *interpreter.DictionaryValue:
				_ = inspectedValue.Entries()
			}
			return true
		},
	)

	m.addKnownContainerStaticTypes(rootValue, owner, key)

	// Infer the static types for array values and dictionary values

	if m.accounts != nil {
		err = m.inferContainerStaticTypes(rootValue)
		if err != nil {
			return nil, false, err
		}

		// If the migrated value is a composite and its type is missing,
		// then delete it

		if compositeValue, ok := rootValue.(*interpreter.CompositeValue); ok {
			if _, ok := m.brokenTypeIDs[compositeValue.TypeID()]; ok {
				// TODO: only when type is literally missing?
				//   cause == brokenTypeCauseMissingCompositeType

				m.Log.Warn().
					Str("owner", owner.String()).
					Str("key", key).
					Msgf(
						"DELETING composite value with missing type: %s",
						compositeValue.String(),
					)

				return nil, false, nil
			}
		}
	}

	// Check static types of arrays and dictionaries

	interpreter.InspectValue(
		rootValue,
		func(inspectedValue interpreter.Value) bool {

			// NOTE: important: walking of siblings continues
			// after setting an error and returning false (to stop walking),
			// so don't overwrite a potentially already set error
			if err != nil {
				return false
			}
			switch inspectedValue := inspectedValue.(type) {
			case *interpreter.ArrayValue:

				if !m.arrayHasStaticType(inspectedValue) {

					err = inferArrayStaticType(inspectedValue, nil)
					if err != nil {
						return false
					}

					m.Log.Warn().
						Str("key", key).
						Str("owner", owner.String()).
						Str("rootValue", rootValue.String()).
						Msgf(
							"inferred array static type %s from contents: %s",
							inspectedValue.StaticType(),
							inspectedValue,
						)
				}

			case *interpreter.DictionaryValue:

				if !m.dictionaryHasStaticType(inspectedValue) {

					err = inferDictionaryStaticType(inspectedValue, nil)
					if err != nil {
						return false
					}

					m.Log.Warn().
						Str("key", key).
						Str("owner", owner.String()).
						Str("rootValue", rootValue.String()).
						Msgf(
							"inferred dictionary static type %s from contents: %s",
							inspectedValue.StaticType(),
							inspectedValue,
						)
				}
			}

			return true
		},
	)
	if err != nil {
		return nil, false, err
	}

	// Encode the value using the new encoder

	newData, deferrals, err := interpreter.EncodeValue(rootValue, path, true, nil)
	if err != nil {
		log.Err(err).
			Str("key", key).
			Str("owner", owner.String()).
			Str("rootValue", rootValue.String()).
			Msg("failed to encode value")
		return data, false, nil
	}

	// Encoding should not provide any deferred values or deferred moves

	if len(deferrals.Values) > 0 {
		return nil, false, fmt.Errorf(
			"re-encoding produced deferred values:\n%s\n",
			rootValue,
		)
	}

	if len(deferrals.Moves) > 0 {
		return nil, false, fmt.Errorf(
			"re-encoding produced deferred moves:\n%s\n",
			rootValue,
		)
	}

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				m.Log.Err(err).
					Str("key", key).
					Str("owner", owner.String()).
					Msgf(
						"failed to decode re-encoded value:\n\nvalue: %s\n\nnewData:\n%s\n\noldData:\n%s\n",
						rootValue,
						hex.Dump(newData),
						hex.Dump(data),
					)
			} else {
				m.Log.Error().
					Str("key", key).
					Str("owner", owner.String()).
					Msgf(
						"failed to decode re-encoded value: %s\n\nvalue: %s\n\nnewData:\n%s\n\noldData:\n%s\n",
						r,
						rootValue,
						hex.Dump(newData),
						hex.Dump(data),
					)
			}

			panic(r)
		}
	}()

	// Sanity check: Decode the newly encoded data again
	// and compare it to the initially decoded value

	newRootValue, err := interpreter.DecodeValue(
		newData,
		&owner,
		path,
		interpreter.CurrentEncodingVersion,
		nil,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"failed to decode re-encoded value: %w\n%s\n",
			err, rootValue,
		)
	}

	equatableValue, ok := rootValue.(interpreter.EquatableValue)
	if !ok {
		return nil, false, fmt.Errorf(
			"cannot compare unequatable %[1]T\n%[1]s\n",
			rootValue,
		)
	}

	if !equatableValue.Equal(newRootValue, nil, false) {
		return nil, false, fmt.Errorf(
			"values are unequal:\n%s\n%s\n",
			rootValue,
			newRootValue,
		)
	}

	return newData, true, nil
}

func (m StorageFormatV5Migration) arrayHasStaticType(arrayValue *interpreter.ArrayValue) bool {
	return arrayValue.Type != nil &&
		arrayValue.Type.ElementType() != nil
}

func (m StorageFormatV5Migration) dictionaryHasStaticType(
	dictionaryValue *interpreter.DictionaryValue,
) bool {
	return m.arrayHasStaticType(dictionaryValue.Keys()) &&
		dictionaryValue.Type.KeyType != nil &&
		dictionaryValue.Type.ValueType != nil
}

func (m StorageFormatV5Migration) inferContainerStaticTypes(
	rootValue interpreter.Value,
) error {

	var err error

	// Start with composite types and use fields' types to infer values' types

	interpreter.InspectValue(
		rootValue,
		func(inspectedValue interpreter.Value) bool {

			// NOTE: important: walking of siblings continues
			// after setting an error and returning false (to stop walking),
			// so don't overwrite a potentially already set error
			if err != nil {
				return false
			}

			compositeValue, ok := inspectedValue.(*interpreter.CompositeValue)
			if !ok {
				// The inspected value is not a composite value,
				// continue inspecting other values
				return true
			}

			// If the inference for the composite's type failed before,
			// then ignore this composite and continue inspecting other values

			typeID := compositeValue.TypeID()
			_, isBroken := m.brokenTypeIDs[typeID]
			if isBroken {
				return true
			}

			var program *interpreter.Program
			program, err = m.loadProgram(compositeValue.Location())
			if err != nil {
				var parsingCheckingError *runtime.ParsingCheckingError
				if !errors.As(err, &parsingCheckingError) {
					// If loading the program failed and it was not "just" a parsing / checking error,
					// then something else is wrong, so abort the migration
					return false
				}

				// If the program for the composite's type could not be parsed or checked,
				// report it, prevent a re-parse and re-check, and continue inspecting other values

				m.Log.Err(err).
					Str("typeID", string(typeID)).
					Msg("failed to parse and check program")

				m.brokenTypeIDs[typeID] = brokenTypeCauseParsingCheckingError

				err = nil
				return true
			}

			compositeType := program.Elaboration.CompositeTypes[typeID]
			if compositeType == nil {

				// If the composite type is missing,
				// report it, prevent a re-parse and re-check, and continue inspecting other values

				m.Log.Error().
					Str("typeID", string(typeID)).
					Msg("missing composite type")

				m.brokenTypeIDs[typeID] = brokenTypeCauseMissingCompositeType

				return true
			}

			var fieldsToDelete []string

			fields := compositeValue.Fields()
			for pair := fields.Oldest(); pair != nil; pair = pair.Next() {
				fieldName := pair.Key
				fieldValue := pair.Value

				member, ok := compositeType.Members.Get(fieldName)
				if !ok {
					m.Log.Warn().
						Msgf("missing type for field: %s.%s", typeID, fieldName)

					// TODO: OK?
					// If the type info for the field is missing,
					// then delete the field contents

					fieldsToDelete = append(fieldsToDelete, fieldName)
					continue
				}

				fieldType := interpreter.ConvertSemaToStaticType(member.TypeAnnotation.Type)

				err = inferContainerStaticType(fieldValue, fieldType)
				if err != nil {

					// If the container type cannot be inferred using the field type,
					// report it and continue inferring other fields

					m.Log.Err(err).
						Str("typeID", string(typeID)).
						Str("fieldType", fieldType.String()).
						Str("fieldValue", fieldValue.String()).
						Msg("failed to infer container type based on field type")

					err = nil

					// The field's type may have been updated from an array to a dictionary, and vice versa.
					// If the field value is an empty array or dictionary, replace the value with a new
					// empty container that has the expected type

					var newFieldValue interpreter.Value

					switch fieldValue := fieldValue.(type) {
					case *interpreter.ArrayValue:
						if dictionaryType, ok := fieldType.(interpreter.DictionaryStaticType); ok &&
							fieldValue.Count() == 0 {

							newFieldValue = interpreter.NewDictionaryValueUnownedNonCopying(dictionaryType)
						}
					case *interpreter.DictionaryValue:
						if arrayStaticType, ok := fieldType.(interpreter.ArrayStaticType); ok &&
							fieldValue.Count() == 0 {

							newFieldValue = interpreter.NewArrayValueUnownedNonCopying(arrayStaticType)
						}
					}

					if newFieldValue != nil {
						fields.Set(fieldName, newFieldValue)

						m.Log.Warn().
							Str("typeID", string(typeID)).
							Str("fieldType", fieldType.String()).
							Str("oldFieldValue", fieldValue.String()).
							Str("newFieldValue", newFieldValue.String()).
							Msg("replaced incorrect empty container value")
					}
				}
			}

			for _, fieldName := range fieldsToDelete {
				m.Log.Warn().
					Msgf("removing field with missing type: %s.%s", typeID, fieldName)

				fields.Delete(fieldName)
			}

			return true
		},
	)

	return err
}

var testnetNFTLocation = func() common.Location {
	address, err := common.HexToAddress("631e88ae7f1d7c20")
	if err != nil {
		panic(err)
	}
	return common.AddressLocation{
		Address: address,
		Name:    "NonFungibleToken",
	}
}()

func (m StorageFormatV5Migration) addKnownContainerStaticTypes(
	value interpreter.Value,
	owner common.Address,
	key string,
) {

	interpreter.InspectValue(
		value,
		func(inspectedValue interpreter.Value) (cont bool) {
			cont = true

			switch inspectedValue := inspectedValue.(type) {
			case *interpreter.CompositeValue:
				switch inspectedValue.QualifiedIdentifier() {
				case "FlowIDTableStaking":

					if !hasAnyLocationAddress(
						inspectedValue,
						"dee35303492e5a0b",
						"1864ff317a35af46",
						"16a5fe3b527633d4",
						"9798362e92e5539a",
					) {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"nodes",
						interpreter.PrimitiveStaticTypeString,
						interpreter.CompositeStaticType{
							Location:            inspectedValue.Location(),
							QualifiedIdentifier: "FlowIDTableStaking.NodeRecord",
						},
						owner,
						key,
					)

					for _, fieldName := range []string{
						"minimumStakeRequired",
						"totalTokensStakedByNodeType",
						"rewardRatios",
					} {
						m.addDictionaryFieldType(
							inspectedValue,
							fieldName,
							interpreter.PrimitiveStaticTypeUInt8,
							interpreter.PrimitiveStaticTypeUFix64,
							owner,
							key,
						)
					}

				case "MessageBoard":

					if !hasAnyLocationAddress(
						inspectedValue,
						"ac98da57ce4dd4ef",
					) {
						return
					}

					m.addArrayFieldType(
						inspectedValue,
						"posts",
						interpreter.VariableSizedStaticType{
							Type: interpreter.CompositeStaticType{
								Location:            inspectedValue.Location(),
								QualifiedIdentifier: "MessageBoard.Post",
							},
						},
						owner,
						key,
					)

				case "FlowIDTableStaking.NodeRecord":

					if !hasAnyLocationAddress(
						inspectedValue,
						"ecda6c5746d5bdf0",
						"f1a43bfd1354c9b8",
						"e94f751ba094ef6a",
						"76d9ea44cef09e20",
					) {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"delegators",
						interpreter.PrimitiveStaticTypeUInt32,
						interpreter.CompositeStaticType{
							Location:            inspectedValue.Location(),
							QualifiedIdentifier: "FlowIDTableStaking.DelegatorRecord",
						},
						owner,
						key,
					)

				case "KittyItemsMarket.Collection":

					if !hasAnyLocationAddress(inspectedValue,
						"fcceff21d9532b58",
						"172be932e9cd0a8f",
					) {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"saleOffers",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.CompositeStaticType{
							Location:            inspectedValue.Location(),
							QualifiedIdentifier: "KittyItemsMarket.SaleOffer",
						},
						owner,
						key,
					)

				case "FlowAssetsMarket.Collection":

					// Probably based on KittyItemsMarket

					if !hasAnyLocationAddress(inspectedValue,
						"108040d5a5922573",
					) {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"saleOffers",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.CompositeStaticType{
							Location:            inspectedValue.Location(),
							QualifiedIdentifier: "FlowAssetsMarket.SaleOffer",
						},
						owner,
						key,
					)

				case "RecordShop.Collection":

					// Probably based on KittyItemsMarket

					if !hasAnyLocationAddress(inspectedValue, "7352d990d2addd95") {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"saleOffers",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.CompositeStaticType{
							Location:            inspectedValue.Location(),
							QualifiedIdentifier: "RecordShop.SaleOffer",
						},
						owner,
						key,
					)

				case "LikeNastyaItemsMarket.Collection":

					// Probably based on KittyItemsMarket

					if !hasAnyLocationAddress(inspectedValue, "9f3e19cda04154fc") {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"saleOffers",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.CompositeStaticType{
							Location:            inspectedValue.Location(),
							QualifiedIdentifier: "LikeNastyaItemsMarket.SaleOffer",
						},
						owner,
						key,
					)

				case "TopShot.Collection",
					"KittyItems.Collection":

					// NOTE: not checking owner,
					// assume this an unmodified copy

					m.addDictionaryFieldType(
						inspectedValue,
						"ownedNFTs",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.InterfaceStaticType{
							Location:            testnetNFTLocation,
							QualifiedIdentifier: "NonFungibleToken.NFT",
						},
						owner,
						key,
					)

				case "Art.Collection",
					"FlowAssets.Collection",
					"TRART.Collection",
					"TRARTNFTTest1.Collection":

					if !hasAnyLocationAddress(
						inspectedValue,
						"f79ee844bfa76528",
						"fcceff21d9532b58",
						"0f349bd983379597",
						"1ff7e32d71183db0",
						"b4544c1d61e8f500",
						"dbe2ee1818a49053",
						"172be932e9cd0a8f",
						"92d59da2af37f015",
						"566c813b3632783e",
						"b4544c1d61e8f500",
						"6358f863215dda14",
					) {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"ownedNFTs",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.InterfaceStaticType{
							Location:            testnetNFTLocation,
							QualifiedIdentifier: "NonFungibleToken.NFT",
						},
						owner,
						key,
					)

				case "StargateNFT.StargateMasterCollection":

					if !hasAnyLocationAddress(
						inspectedValue,
						"dd9ed5717c7d1af1",
					) {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"ownedNFTs",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.InterfaceStaticType{
							Location:            testnetNFTLocation,
							QualifiedIdentifier: "NonFungibleToken.NFT",
						},
						owner,
						key,
					)

					m.addDictionaryFieldType(
						inspectedValue,
						"nonces",
						interpreter.PrimitiveStaticTypeAddress,
						interpreter.PrimitiveStaticTypeUInt64,
						owner,
						key,
					)

				case "HastenScript.Collection":

					if !hasAnyLocationAddress(
						inspectedValue,
						"f8d51e8d9f1ceb86",
					) {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"ownedScripts",
						interpreter.PrimitiveStaticTypeUInt256,
						interpreter.CompositeStaticType{
							Location:            inspectedValue.Location(),
							QualifiedIdentifier: "HastenScript.Script",
						},
						owner,
						key,
					)

				case "HastenScript.Script":

					if !hasAnyLocationAddress(
						inspectedValue,
						"f8d51e8d9f1ceb86",
					) {
						return
					}

					for _, fieldName := range []string{"code", "environment"} {
						m.addArrayFieldType(
							inspectedValue,
							fieldName,
							interpreter.VariableSizedStaticType{
								Type: interpreter.PrimitiveStaticTypeUInt8,
							},
							owner,
							key,
						)
					}

				case "LikeNastyaItems.Collection":

					// Likely https://medium.com/pinata/how-to-create-nfts-like-nba-top-shot-with-flow-and-ipfs-701296944bf

					if !hasAnyLocationAddress(inspectedValue, "9F3E19CDA04154FC") {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"ownedNFTs",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.InterfaceStaticType{
							Location:            testnetNFTLocation,
							QualifiedIdentifier: "NonFungibleToken.NFT",
						},
						owner,
						key,
					)

					m.addDictionaryFieldType(
						inspectedValue,
						"metadataObjs",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.DictionaryStaticType{
							KeyType:   interpreter.PrimitiveStaticTypeString,
							ValueType: interpreter.PrimitiveStaticTypeString,
						},
						owner,
						key,
					)

				case "Versus.DropCollection":
					if !hasAnyLocationAddress(
						inspectedValue,
						"1ff7e32d71183db0",
						"467694dd28ef0a12",
					) {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"drops",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.CompositeStaticType{
							Location:            inspectedValue.Location(),
							QualifiedIdentifier: "Versus.Drop",
						},
						owner,
						key,
					)

				case "Auction.AuctionCollection":

					if !hasAnyLocationAddress(inspectedValue, "1ff7e32d71183db0") {
						return
					}

					m.addDictionaryFieldType(
						inspectedValue,
						"auctionItems",
						interpreter.PrimitiveStaticTypeUInt64,
						interpreter.InterfaceStaticType{
							Location:            inspectedValue.Location(),
							QualifiedIdentifier: "Auction.AuctionItem",
						},
						owner,
						key,
					)

				case "Connections.Base":
					if !hasAnyLocationAddress(
						inspectedValue,
						"7ed3a3ff81329797",
					) {
						return
					}

					for _, fieldName := range []string{"followers", "following"} {

						m.addDictionaryFieldType(
							inspectedValue,
							fieldName,
							interpreter.PrimitiveStaticTypeAddress,
							interpreter.PrimitiveStaticTypeBool,
							owner,
							key,
						)
					}
				}
			}

			return
		},
	)
}

func (m StorageFormatV5Migration) addDictionaryFieldType(
	value *interpreter.CompositeValue,
	fieldName string,
	keyType interpreter.StaticType,
	valueType interpreter.StaticType,
	owner common.Address,
	key string,
) {
	fieldValue, ok := value.Fields().Get(fieldName)
	if !ok {
		return
	}

	dictionaryValue, ok := fieldValue.(*interpreter.DictionaryValue)
	if !ok {
		return
	}

	dictionaryValue.Keys().Type = interpreter.VariableSizedStaticType{
		Type: keyType,
	}

	dictionaryValue.Type = interpreter.DictionaryStaticType{
		KeyType:   keyType,
		ValueType: valueType,
	}

	m.Log.Info().
		Str("owner", owner.String()).
		Str("key", key).
		Msgf(
			"added known static type %s to dictionary: %s",
			dictionaryValue.Type,
			dictionaryValue.String(),
		)
}

func (m StorageFormatV5Migration) addArrayFieldType(
	value *interpreter.CompositeValue,
	fieldName string,
	arrayType interpreter.ArrayStaticType,
	owner common.Address,
	key string,
) {
	fieldValue, ok := value.Fields().Get(fieldName)
	if !ok {
		return
	}

	arrayValue, ok := fieldValue.(*interpreter.ArrayValue)
	if !ok {
		return
	}

	arrayValue.Type = arrayType

	m.Log.Info().
		Str("owner", owner.String()).
		Str("key", key).
		Msgf(
			"added known static type %s to array: %s",
			arrayValue.Type,
			arrayValue.String(),
		)
}

var contractValueChildKeyRegexp = regexp.MustCompile("^contract\x1f([^\x1f]+)\x1f.+")

func (m StorageFormatV5Migration) isOrphanContactValueChildKey(
	key []byte,
	owner flow.Address,
) bool {
	contractName := getContractValueChildKeyContractName(key)
	return contractName != "" &&
		!m.contractExists(owner, contractName)
}

func (m StorageFormatV5Migration) contractExists(
	owner flow.Address,
	contractName string,
) bool {
	contractNames, err := m.accounts.GetContractNames(owner)
	if err != nil {
		m.Log.Err(err).Msgf("failed to get contract names for account %s", owner.String())
		return false
	}

	i := sort.SearchStrings(contractNames, contractName)
	return i != len(contractNames) && contractNames[i] == contractName
}

func getContractValueChildKeyContractName(key []byte) string {
	matches := contractValueChildKeyRegexp.FindSubmatch(key)
	if len(matches) == 0 {
		return ""
	}
	return string(matches[1])
}

func hasAnyLocationAddress(value *interpreter.CompositeValue, hexAddresses ...string) bool {
	location := value.Location()
	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		return false
	}

	for _, hexAddress := range hexAddresses {
		address, err := common.HexToAddress(hexAddress)
		if err != nil {
			return false
		}
		if addressLocation.Address == address {
			return true
		}
	}

	return false
}

func inferContainerStaticType(value interpreter.Value, t interpreter.StaticType) error {

	// Only infer static type for arrays and dictionaries

	switch value := value.(type) {
	case *interpreter.ArrayValue:
		err := inferArrayStaticType(value, t)
		if err != nil {
			return err
		}

	case *interpreter.DictionaryValue:
		err := inferDictionaryStaticType(value, t)
		if err != nil {
			return err
		}
	}

	return nil
}

func inferArrayStaticType(value *interpreter.ArrayValue, t interpreter.StaticType) error {

	if t == nil {
		if value.Count() == 0 {
			return fmt.Errorf("cannot infer static type for empty array value")
		}

		var elementType interpreter.StaticType

		for _, element := range value.Elements() {
			if elementType == nil {
				elementType = element.StaticType()
			} else if !element.StaticType().Equal(elementType) {
				return fmt.Errorf("cannot infer static type for array with mixed elements")
			}
		}

		// TODO: infer element type to AnyStruct or AnyResource based on kinds of elements instead?
		value.Type = interpreter.VariableSizedStaticType{
			Type: elementType,
		}
	} else {

		switch arrayType := t.(type) {
		case interpreter.VariableSizedStaticType:
			value.Type = arrayType

		case interpreter.ConstantSizedStaticType:
			value.Type = arrayType

		default:
			switch t {
			case interpreter.PrimitiveStaticTypeAnyStruct,
				interpreter.PrimitiveStaticTypeAnyResource:

				value.Type = interpreter.VariableSizedStaticType{
					Type: t,
				}

			default:
				return fmt.Errorf(
					"failed to infer static type for array value and given type %s: %s",
					t, value,
				)
			}
		}
	}

	// Recursively infer type for array elements

	elementType := value.Type.ElementType()

	for _, element := range value.Elements() {
		err := inferContainerStaticType(element, elementType)
		if err != nil {
			return err
		}
	}

	return nil
}

func inferDictionaryStaticType(value *interpreter.DictionaryValue, t interpreter.StaticType) error {
	entries := value.Entries()

	if t == nil {
		// NOTE: use entries.Len() instead of Count, because Count() > 0 && entries.Len() == 0 means
		// the dictionary has deferred (separately stored) values, and we cannot get the types of those values
		if entries.Len() == 0 {
			return fmt.Errorf("cannot infer static type for empty dictionary value: %s", value.String())
		} else {

			var keyType interpreter.StaticType
			for _, key := range value.Keys().Elements() {
				if keyType == nil {
					keyType = key.StaticType()
				} else if !key.StaticType().Equal(keyType) {
					return fmt.Errorf("cannot infer key static type for dictionary with mixed type keys")
				}
			}

			var valueType interpreter.StaticType
			for pair := entries.Oldest(); pair != nil; pair = pair.Next() {
				if valueType == nil {
					valueType = pair.Value.StaticType()
				} else if !pair.Value.StaticType().Equal(valueType) {
					return fmt.Errorf("cannot infer value static type for dictionary with mixed type values")
				}
			}

			// TODO: infer value type to AnyStruct or AnyResource based on kinds of values instead?
			value.Type = interpreter.DictionaryStaticType{
				KeyType:   keyType,
				ValueType: valueType,
			}
		}
	} else {

		if dictionaryType, ok := t.(interpreter.DictionaryStaticType); ok {
			value.Type = dictionaryType
		} else {
			switch t {
			case interpreter.PrimitiveStaticTypeAnyStruct,
				interpreter.PrimitiveStaticTypeAnyResource:

				value.Type = interpreter.DictionaryStaticType{
					KeyType:   interpreter.PrimitiveStaticTypeAnyStruct,
					ValueType: t,
				}

			default:
				return fmt.Errorf(
					"failed to infer static type for dictionary value and given type %s: %s",
					t, value,
				)
			}
		}
	}

	// Recursively infer type for dictionary keys and values

	err := inferContainerStaticType(
		value.Keys(),
		interpreter.VariableSizedStaticType{
			Type: value.Type.KeyType,
		},
	)
	if err != nil {
		return err
	}

	for pair := entries.Oldest(); pair != nil; pair = pair.Next() {
		err := inferContainerStaticType(
			pair.Value,
			value.Type.ValueType,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m StorageFormatV5Migration) loadProgram(
	location common.Location,
) (
	*interpreter.Program,
	error,
) {
	program, _, ok := m.programs.Get(location)
	if ok {
		return program, nil
	}

	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, fmt.Errorf(
			"cannot load program for unsupported non-address location: %s",
			addressLocation,
		)
	}

	contractCode, err := m.accounts.GetContract(
		addressLocation.Name,
		flow.Address(addressLocation.Address),
	)
	if err != nil {
		return nil, err
	}

	rt := runtime.NewInterpreterRuntime()
	program, err = rt.ParseAndCheckProgram(
		contractCode,
		runtime.Context{
			Interface: migrationRuntimeInterface{m.accounts, m.programs},
			Location:  location,
		},
	)
	if err != nil {
		return nil, err
	}

	m.programs.Set(location, program, nil)

	return program, nil
}

type migrationRuntimeInterface struct {
	accounts *state.Accounts
	programs *programs.Programs
}

func (m migrationRuntimeInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {

	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location (`import Crypto`),
	// then return a single resolved location which declares all identifiers.
	if !isAddress {
		return []runtime.ResolvedLocation{
			{
				Location:    location,
				Identifiers: identifiers,
			},
		}, nil
	}

	// if the location is an address,
	// and no specific identifiers where requested in the import statement,
	// then fetch all identifiers at this address
	if len(identifiers) == 0 {
		address := flow.Address(addressLocation.Address)

		contractNames, err := m.accounts.GetContractNames(address)
		if err != nil {
			return nil, fmt.Errorf("ResolveLocation failed: %w", err)
		}

		// if there are no contractNames deployed,
		// then return no resolved locations
		if len(contractNames) == 0 {
			return nil, nil
		}

		identifiers = make([]ast.Identifier, len(contractNames))

		for i := range identifiers {
			identifiers[i] = runtime.Identifier{
				Identifier: contractNames[i],
			}
		}
	}

	// return one resolved location per identifier.
	// each resolved location is an address contract location
	resolvedLocations := make([]runtime.ResolvedLocation, len(identifiers))
	for i := range resolvedLocations {
		identifier := identifiers[i]
		resolvedLocations[i] = runtime.ResolvedLocation{
			Location: common.AddressLocation{
				Address: addressLocation.Address,
				Name:    identifier.Identifier,
			},
			Identifiers: []runtime.Identifier{identifier},
		}
	}

	return resolvedLocations, nil
}

func (m migrationRuntimeInterface) GetCode(location runtime.Location) ([]byte, error) {
	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("GetCode failed: expected AddressLocation")
	}

	add, err := m.accounts.GetContract(contractLocation.Name, flow.Address(contractLocation.Address))
	if err != nil {
		return nil, fmt.Errorf("GetCode failed: %w", err)
	}

	return add, nil
}

func (m migrationRuntimeInterface) GetProgram(location runtime.Location) (*interpreter.Program, error) {
	program, _, ok := m.programs.Get(location)
	if ok {
		return program, nil
	}

	return nil, nil
}

func (m migrationRuntimeInterface) SetProgram(location runtime.Location, program *interpreter.Program) error {
	m.programs.Set(location, program, nil)
	return nil
}

func (m migrationRuntimeInterface) GetValue(_, _ []byte) (value []byte, err error) {
	panic("unexpected GetValue call")
}

func (m migrationRuntimeInterface) SetValue(_, _, _ []byte) (err error) {
	panic("unexpected SetValue call")
}

func (m migrationRuntimeInterface) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	panic("unexpected CreateAccount call")
}

func (m migrationRuntimeInterface) AddEncodedAccountKey(_ runtime.Address, _ []byte) error {
	panic("unexpected AddEncodedAccountKey call")
}

func (m migrationRuntimeInterface) RevokeEncodedAccountKey(_ runtime.Address, _ int) (publicKey []byte, err error) {
	panic("unexpected RevokeEncodedAccountKey call")
}

func (m migrationRuntimeInterface) AddAccountKey(
	_ runtime.Address,
	_ *runtime.PublicKey,
	_ runtime.HashAlgorithm,
	_ int,
) (*runtime.AccountKey, error) {
	panic("unexpected AddAccountKey call")
}

func (m migrationRuntimeInterface) GetAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected GetAccountKey call")
}

func (m migrationRuntimeInterface) RevokeAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected RevokeAccountKey call")
}

func (m migrationRuntimeInterface) UpdateAccountContractCode(_ runtime.Address, _ string, _ []byte) (err error) {
	panic("unexpected UpdateAccountContractCode call")
}

func (m migrationRuntimeInterface) GetAccountContractCode(
	address runtime.Address,
	name string,
) (code []byte, err error) {
	return m.accounts.GetContract(name, flow.Address(address))
}

func (m migrationRuntimeInterface) RemoveAccountContractCode(_ runtime.Address, _ string) (err error) {
	panic("unexpected RemoveAccountContractCode call")
}

func (m migrationRuntimeInterface) GetSigningAccounts() ([]runtime.Address, error) {
	panic("unexpected GetSigningAccounts call")
}

func (m migrationRuntimeInterface) ProgramLog(_ string) error {
	panic("unexpected ProgramLog call")
}

func (m migrationRuntimeInterface) EmitEvent(_ cadence.Event) error {
	panic("unexpected EmitEvent call")
}

func (m migrationRuntimeInterface) ValueExists(_, _ []byte) (exists bool, err error) {
	panic("unexpected ValueExists call")
}

func (m migrationRuntimeInterface) GenerateUUID() (uint64, error) {
	panic("unexpected GenerateUUID call")
}

func (m migrationRuntimeInterface) GetComputationLimit() uint64 {
	panic("unexpected GetComputationLimit call")
}

func (m migrationRuntimeInterface) SetComputationUsed(_ uint64) error {
	panic("unexpected SetComputationUsed call")
}

func (m migrationRuntimeInterface) DecodeArgument(_ []byte, _ cadence.Type) (cadence.Value, error) {
	panic("unexpected DecodeArgument call")
}

func (m migrationRuntimeInterface) GetCurrentBlockHeight() (uint64, error) {
	panic("unexpected GetCurrentBlockHeight call")
}

func (m migrationRuntimeInterface) GetBlockAtHeight(_ uint64) (block runtime.Block, exists bool, err error) {
	panic("unexpected GetBlockAtHeight call")
}

func (m migrationRuntimeInterface) UnsafeRandom() (uint64, error) {
	panic("unexpected UnsafeRandom call")
}

func (m migrationRuntimeInterface) VerifySignature(
	_ []byte,
	_ string,
	_ []byte,
	_ []byte,
	_ runtime.SignatureAlgorithm,
	_ runtime.HashAlgorithm,
) (bool, error) {
	panic("unexpected VerifySignature call")
}

func (m migrationRuntimeInterface) Hash(_ []byte, _ string, _ runtime.HashAlgorithm) ([]byte, error) {
	panic("unexpected Hash call")
}

func (m migrationRuntimeInterface) GetAccountBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountBalance call")
}

func (m migrationRuntimeInterface) GetAccountAvailableBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountAvailableBalance call")
}

func (m migrationRuntimeInterface) GetStorageUsed(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageUsed call")
}

func (m migrationRuntimeInterface) GetStorageCapacity(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageCapacity call")
}

func (m migrationRuntimeInterface) ImplementationDebugLog(_ string) error {
	panic("unexpected ImplementationDebugLog call")
}

func (m migrationRuntimeInterface) ValidatePublicKey(_ *runtime.PublicKey) (bool, error) {
	panic("unexpected ValidatePublicKey call")
}
