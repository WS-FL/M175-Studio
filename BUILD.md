# Build on macOS

The corresponding English source has been prepared locally and has not yet been published. These instructions describe the verified local build and will be usable after source publication.

Requirements: macOS, Xcode Command Line Tools, Go 1.23.2 or a compatible version, and Python 3. The app targets macOS 12 and includes Apple Silicon and Intel executables.

```sh
cd engine
go test -race -count=1 ./...
cd ..
chmod +x build-macos.sh
./build-macos.sh
```

The output is `mac-build/M175 Studio.app`. The script builds both Go engines with `-trimpath`, compiles the native host against the installed Apple SDK, and applies ad-hoc signatures. It verifies bundle signatures but does not perform Developer ID signing or notarization.

For a distributable ZIP, place the app and `INSTALL.txt` in a folder and compress it with macOS `ditto`:

```sh
mkdir -p release/M175-Studio-English
cp -R "mac-build/M175 Studio.app" release/M175-Studio-English/
cp INSTALL.txt release/M175-Studio-English/
ditto -c -k --keepParent release/M175-Studio-English M175_Studio_Mac_0.2.2_English.zip
shasum -a 256 M175_Studio_Mac_0.2.2_English.zip
```

This English revision is built from the recovered scan core and native source, translated and updated to preserve the compact 0.2.2 scan header. The original 0.2.1/0.2.2 patch sources were not recovered, so it is not an exact reproduction of the original Chinese package. On a real Mac, the native host uses installed Apple framework headers, avoiding the minimal cross-build ABI declaration issue reported for the old 0.2.0 package.

Legacy cross-build helpers are retained for reference. The supported recipe above uses the real Apple SDK and does not redistribute Apple SDK files or framework binaries. Preserve NOTICE.txt and GO-LICENSE.txt when distributing an app.
