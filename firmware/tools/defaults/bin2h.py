#!/usr/bin/env python3
"""
Convert binary file to C++ PROGMEM header file.
Output format: const uint8_t data[] PROGMEM = { ... };
"""

import sys
import os

def bin_to_h(input_file, output_file, var_name):
    """Convert binary file to PROGMEM C array."""
    with open(input_file, 'rb') as f:
        data = f.read()

    if not data:
        print(f"Error: Empty input file {input_file}")
        sys.exit(1)

    # Get file size
    file_size = len(data)
    input_basename = os.path.basename(input_file)

    # Generate header
    header = f"""// Auto-generated from {input_basename}
// Size: {file_size} bytes

#ifndef {var_name.upper()}_H
#define {var_name.upper()}_H

#include <Arduino.h>

const uint8_t {var_name}[] PROGMEM = {{
"""

    # Write data as hex bytes (16 per line for readability)
    for i, byte in enumerate(data):
        if i % 16 == 0:
            header += "  "
        header += f"0x{byte:02x},"
        if (i + 1) % 16 == 0:
            header += "\n"

    # Close last line if needed
    if len(data) % 16 != 0:
        header += "\n"

    footer = f"""
}};

constexpr size_t {var_name}_size = {file_size};

#endif // {var_name.upper()}_H
"""

    # Write header file
    with open(output_file, 'w') as f:
        f.write(header)
        f.write(footer)

    print(f"Generated: {output_file} ({var_name}[] = {file_size} bytes)")
    return True

if __name__ == "__main__":
    if len(sys.argv) != 4:
        print("Usage: bin2h.py <input.bin.rle> <output.h> <var_name>")
        sys.exit(1)

    input_file = sys.argv[1]
    output_file = sys.argv[2]
    var_name = sys.argv[3]

    try:
        bin_to_h(input_file, output_file, var_name)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)
