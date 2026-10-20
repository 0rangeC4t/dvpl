# Project rules

- DVPL is payload followed by a 20-byte little-endian footer: `uint32 originalSize | uint32 packedSize | uint32 CRC32-IEEE of payload | uint32 type | "DVPL"`.
- Type labels follow compression effort: 0 none, 1 regular LZ4, 2 LZ4_HC. This mapping is not verified against real game files.
- Keep the type-number mapping in one function. When reading, 0 is raw; 1 and 2 are LZ4 blocks; all other values are errors.
- Dependencies are limited to the standard library and `github.com/pierrec/lz4/v4`. Do not add cobra, survey, or klauspost/crc32.
- All DVPL validation belongs in `internal/dvpl`, not the codec.
- Keep originals by default. Write files atomically using a temporary file in the same directory followed by rename; one file's error must not stop a batch.
- Generate test data in tests; never copy Tankerch's `test_files`.
- Preserve licenses: Apache-2.0 and NOTICE (qirashi), MIT (Tankerch), BSD-3 (pierrec).
- Plan: 1a library; 1b `internal/core`; 1c `cmd/dvpl`, README, LICENSE/NOTICE; step 2 cgo backend; step 3 TUI.
