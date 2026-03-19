WISP   = ../../../api/wisp
OUTDIR = ../../src/assets/error
TMPDIR = /tmp/wisp-defaults

DISPLAY_epd7in3e  = ws7in3e
DISPLAY_epd4in0e  = ws4in0e
DISPLAY_epd13in3e = ws13in3e
DISPLAY_epd13in3k = ws13in3k

SIZE_epd7in3e  = 800 480
SIZE_epd4in0e  = 600 400
SIZE_epd13in3e = 1200 1600
SIZE_epd13in3k = 960 680

DEVICES = epd7in3e epd4in0e epd13in3e epd13in3k

.PRECIOUS: $(TMPDIR)/%.png

.PHONY: all clean png-check reduced-png-check
all: $(addprefix $(OUTDIR)/,$(addsuffix .bin.rle.h,$(DEVICES)))

# Generate intermediate PNG files without conversion (for inspection)
png-check:
	rm -f $(addprefix $(TMPDIR)/,$(addsuffix .png,$(DEVICES)))
	@$(MAKE) $(addprefix $(TMPDIR)/,$(addsuffix .png,$(DEVICES)))
	@echo "Capture PNG files ready in $(TMPDIR)/ for inspection"

# Generate reduced color PNG files (for inspection)
reduced-png-check:
	rm -f $(addprefix $(TMPDIR)/,$(addsuffix -reduced.png,$(DEVICES)))
	@$(MAKE) $(addprefix $(TMPDIR)/,$(addsuffix -reduced.png,$(DEVICES)))
	@echo "Reduced PNG files ready in $(TMPDIR)/ for inspection"

$(TMPDIR):
	mkdir -p $@ $(OUTDIR)

$(TMPDIR)/%.png: src/%.html | $(TMPDIR)
	node capture.mjs $< $(SIZE_$*) $@

# Reduced color PNG (for inspection)
$(TMPDIR)/%-reduced.png: $(TMPDIR)/%.png
	$(WISP) image convert $< --display $(DISPLAY_$*) -f png -c simple --output $@

# Binary format (intermediate output)
$(OUTDIR)/%.bin: $(TMPDIR)/%.png
	$(WISP) image convert $< --display $(DISPLAY_$*) -f bin -c simple --output $@

# RLE compression (final output)
$(OUTDIR)/%.bin.rle: $(OUTDIR)/%.bin
	python3 encode_rle.py $< $@

# Generate C++ PROGMEM header from RLE data
$(OUTDIR)/%.bin.rle.h: $(OUTDIR)/%.bin.rle
	python3 bin2h.py $< $@ error_screen_$*

clean:
	rm -rf $(TMPDIR) $(OUTDIR)/*.bin $(OUTDIR)/*.bin.rle $(OUTDIR)/*.bin.rle.h
