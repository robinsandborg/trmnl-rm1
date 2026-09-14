# Display pixel baseline

The input and golden files were generated before extraction from committed `54f2fdd` using Go 1.26.2 on Linux. The generator called the original `prepareLandscapeImage` and `applyImageRotation`; no renderer commands or device operations were involved.

`cases.json` specifies the input, dimensions, effective rotation, and expected output for each case. The corpus covers landscape and portrait orientation, center crop, up/down scaling, clockwise software rotations, transparent pixels, and PNG/JPEG/GIF/BMP decoding.

Tests compare every decoded RGBA pixel and dimensions. They do not assume a particular PNG compression stream. The explicit primary-color luminance test also checks the legacy grayscale values independently of the golden files. Fixtures are not regenerated during tests and should change only for an approved behavior migration.
