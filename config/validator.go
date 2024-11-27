package config

import (
	"github.com/mrmikeo/Xpense/config/flags"
	"github.com/pkg/errors"
	cli "gopkg.in/urfave/cli.v1"

	"github.com/Fantom-foundation/lachesis-base/inter/idx"

	"github.com/mrmikeo/Xpense/gossip/emitter"
	"github.com/mrmikeo/Xpense/integration/makefakegenesis"
	"github.com/mrmikeo/Xpense/inter/validatorpk"
)



// setValidatorID retrieves the validator ID either from the directly specified
// command line flags or from the keystore if CLI indexed.
func setValidator(ctx *cli.Context, cfg *emitter.Config) error {
	// Extract the current validator address, new flag overriding legacy one
	if ctx.GlobalIsSet(FakeNetFlag.Name) {
		id, num, err := ParseFakeGen(ctx.GlobalString(FakeNetFlag.Name))
		if err != nil {
			return err
		}

		if ctx.GlobalIsSet(flags.ValidatorIDFlag.Name) && id != 0 {
			return errors.New("specified validator ID with both --fakenet and --validator.id")
		}

		cfg.Validator.ID = id
		validators := makefakegenesis.GetFakeValidators(num)
		cfg.Validator.PubKey = validators.Map()[cfg.Validator.ID].PubKey
	}

	if ctx.GlobalIsSet(flags.ValidatorIDFlag.Name) {
		cfg.Validator.ID = idx.ValidatorID(ctx.GlobalInt(flags.ValidatorIDFlag.Name))
	}

	if ctx.GlobalIsSet(flags.ValidatorPubkeyFlag.Name) {
		pk, err := validatorpk.FromString(ctx.GlobalString(flags.ValidatorPubkeyFlag.Name))
		if err != nil {
			return err
		}
		cfg.Validator.PubKey = pk
	}

	if cfg.Validator.ID != 0 && cfg.Validator.PubKey.Empty() {
		return errors.New("validator public key is not set")
	}
	return nil
}
