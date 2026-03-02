#ifndef EPAPER_FACTORY_H
#define EPAPER_FACTORY_H

#include "EPaperDisplay.h"

class EPaperFactory {
public:
    static EPaperDisplay* create();
};

#endif // EPAPER_FACTORY_H
