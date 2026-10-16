package keyrotation

import (
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
)

func TestDiffConfigs(t *testing.T) {
	t.Run("starting_from is cleared on both sides", func(t *testing.T) {
		startingFrom := strfmt.DateTime(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))

		source, target := diffConfigs(
			&Config{Sig: &Rotation{Enabled: true, Cron: "0 0 1 * *", StartingFrom: &startingFrom}},
			&Config{Sig: &Rotation{Enabled: true, Cron: "0 0 1 * *", StartingFrom: &startingFrom}},
		)

		require.Equal(t, &Config{Sig: &Rotation{Enabled: true, Cron: "0 0 1 * *"}}, source)
		require.Equal(t, &Config{Sig: &Rotation{Enabled: true, Cron: "0 0 1 * *"}}, target)
	})

	t.Run("remote use absent locally is dropped", func(t *testing.T) {
		source, target := diffConfigs(
			&Config{Sig: &Rotation{Enabled: true, Cron: "0 0 1 * *"}},
			&Config{
				Sig: &Rotation{Enabled: false, Cron: "0 0 1 * *"},
				Enc: &Rotation{Enabled: true, Cron: "0 0 1 1 *"},
			},
		)

		require.Equal(t, &Config{Sig: &Rotation{Enabled: true, Cron: "0 0 1 * *"}}, source)
		require.Equal(t, &Config{Sig: &Rotation{Enabled: false, Cron: "0 0 1 * *"}}, target)
	})
}
