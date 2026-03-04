#!/usr/bin/env python3
"""
RLE (Run-Length Encoding) compressor for binary files.
Format: [byte_value, count] pairs (each 1 byte)
Max run length: 255
"""

import sys
import struct

def encode_rle(input_file, output_file):
    """Encode binary file using RLE compression."""
    with open(input_file, 'rb') as f:
        data = f.read()

    if not data:
        print(f"Error: Empty input file {input_file}")
        sys.exit(1)

    encoded = bytearray()
    i = 0

    while i < len(data):
        current_byte = data[i]
        count = 1

        # Count consecutive identical bytes (max 255)
        while i + count < len(data) and data[i + count] == current_byte and count < 255:
            count += 1

        # Write: [byte_value][count]
        encoded.append(current_byte)
        encoded.append(count)

        i += count

    # Write compressed file
    with open(output_file, 'wb') as f:
        f.write(encoded)

    original_size = len(data)
    compressed_size = len(encoded)
    ratio = (1 - compressed_size / original_size) * 100

    print(f"RLE Compression: {original_size} → {compressed_size} bytes ({ratio:.1f}% reduction)")
    return True

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: encode_rle.py <input.bin> <output.bin.rle>")
        sys.exit(1)

    input_file = sys.argv[1]
    output_file = sys.argv[2]

    try:
        encode_rle(input_file, output_file)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)
