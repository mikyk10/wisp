#include "EPaperFactory.h"

#ifdef EPD_WAVESHARE_EPD13IN3E
#include "devices/epd/waveshare/epd13in3e/EPaperDisplayImpl.h"
#elif defined(EPD_WAVESHARE_EPD13IN3K)
#include "devices/epd/waveshare/epd13in3k/EPaperDisplayImpl.h"
#elif defined(EPD_WAVESHARE_EPD7IN3E)
#include "devices/epd/waveshare/epd7in3e/EPaperDisplayImpl.h"
#elif defined(EPD_WAVESHARE_EPD4IN0E)
#include "devices/epd/waveshare/epd4in0e/EPaperDisplayImpl.h"
#endif

EPaperDisplay *EPaperFactory::create()
{
#ifdef EPD_WAVESHARE_EPD13IN3E
    return new EPD13In3EImpl();
#elif defined(EPD_WAVESHARE_EPD13IN3K)
    return new EPD13In3KImpl();
#elif defined(EPD_WAVESHARE_EPD7IN3E)
    return new EPD7In3EImpl();
#elif defined(EPD_WAVESHARE_EPD4IN0E)
    return new EPD4InE6Impl();
#else
    Serial.println("[EPaperFactory] No valid model selected!");
    return nullptr;
#endif
}
