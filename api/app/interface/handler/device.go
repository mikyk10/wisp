package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/mikyk10/wisp/app/domain/catalog"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/interface/handler/response"
	"github.com/mikyk10/wisp/app/usecase"

	"github.com/labstack/echo/v5"
)

type DeviceHandler interface {
	ListDevices(*echo.Context) error
	ListDeliveries(*echo.Context) error
}

type deviceHandler struct {
	devuc usecase.DeviceUsecase
}

func NewDeviceHandler(devuc usecase.DeviceUsecase) DeviceHandler {
	return &deviceHandler{devuc: devuc}
}

// ListDevices returns every configured display and what was last delivered to
// it.
//
// Plain JSON rather than the NDJSON the image list uses: this one is bounded by
// the number of panels in service.yaml, so there is nothing to stream.
func (h *deviceHandler) ListDevices(c *echo.Context) error {
	devices, err := h.devuc.ListDevices()
	if err != nil {
		return err
	}

	records := make([]*response.Device, 0, len(devices))
	for _, dev := range devices {
		records = append(records, &response.Device{
			Key:                  dev.Key,
			Name:                 dev.Name,
			Model:                dev.Model,
			Width:                dev.Width,
			Height:               dev.Height,
			Orientation:          dev.Orientation,
			CatalogKeys:          dev.CatalogKeys,
			SleepDurationSeconds: dev.SleepDurationSeconds,
			WakeSchedule:         dev.WakeSchedule,
			LastDeliveredAt:      deliveredAt(dev.LastDeliveredAt),
			RecentDeliveryCount:  dev.RecentDeliveryCount,
			RecentErrorCount:     dev.RecentErrorCount,
		})
	}

	return c.JSON(http.StatusOK, &response.DeviceList{
		RecordingEnabled: h.devuc.RecordingEnabled(),
		RecentWindow:     h.devuc.RecentWindow(),
		Devices:          records,
	})
}

// ListDeliveries returns one display's recent deliveries, newest first.
func (h *deviceHandler) ListDeliveries(c *echo.Context) error {
	displayKey := c.Param("displayKey")

	// An absent or unreadable limit is not an error: zero asks for the whole
	// window, which is what a caller that says nothing gets. The use case
	// clamps it, since the ring size is its to know.
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	entries, err := h.devuc.ListDeliveries(displayKey, limit)
	if err != nil {
		var notFound *catalog.DisplayNotFoundError
		if errors.As(err, &notFound) {
			// Through the central error handler, so the body carries the same
			// trace_id shape as every other error the API returns.
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}

	records := make([]*response.Delivery, 0, len(entries))
	for _, entry := range entries {
		records = append(records, newDelivery(entry))
	}

	return c.JSON(http.StatusOK, &response.DeliveryList{
		DeviceKey:  displayKey,
		Deliveries: records,
	})
}

// deliveredAt renders a delivery time, or nothing at all when there has not
// been one.
func deliveredAt(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	// Forced to UTC and to seconds, the way the image list already renders a
	// timestamp, so a client never has to reconcile two offsets.
	s := t.UTC().Format(time.RFC3339)
	return &s
}

// newDelivery renders one stored delivery.
//
// ImageAvailable is passed straight through. ListByDisplay answers it with a
// LEFT JOIN in the same query that reads the rows, so looking each photograph
// up again here would be an N+1 for something already known.
func newDelivery(entry *repository.DeliveryHistoryEntry) *response.Delivery {
	rec := &response.Delivery{
		DeliveredAt:           entry.DeliveredAt.UTC().Format(time.RFC3339),
		Kind:                  string(entry.Kind),
		RequestedSleepSeconds: entry.SleepSeconds,
		ImageAvailable:        entry.ImageAvailable,
	}

	// Zero is stored when the picture did not come from the catalogue — a
	// colour bar, an error card, a live HTTP fetch — and no images row can have
	// id 0. Null says that; 0 would read as a photograph.
	if entry.ImageID != 0 {
		id := entry.ImageID
		rec.ImageID = &id
	}
	// Empty for every kind but an error card, and passed through as the stored
	// code. Turning it into a sentence belongs to whoever displays it.
	if entry.Reason != "" {
		reason := string(entry.Reason)
		rec.Reason = &reason
	}
	// Empty when nothing consulted a catalogue. Null rather than "" for the same
	// reason as the two above: an empty string in a key field is the
	// absent-value-mistaken-for-a-value case this whole payload avoids.
	if entry.CatalogKey != "" {
		key := entry.CatalogKey
		rec.CatalogKey = &key
	}
	if entry.Source != "" {
		src := entry.Source
		rec.Source = &src
	}

	return rec
}
