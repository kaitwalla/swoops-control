# Claude Development Notes

## Important Conventions

### Version Management
- **ALWAYS** keep the setup script version in sync with the application version
- Location: `setup.sh` line 6: `SETUP_SCRIPT_VERSION="x.y.z"`
- When tagging a new release, update this version number to match
- The setup script version is displayed to users during installation

### Release Process
1. Make your changes
2. Update `setup.sh` version to match the new release tag
3. Build: `make build-server`
4. Commit changes
5. Create and push tag: `git tag vX.Y.Z && git push origin vX.Y.Z`

### Cache Busting
- The UI automatically adds timestamps to setup script URLs to bypass GitHub's CDN cache
- Location: `web/src/components/CreateHostDialog.tsx`
- Format: `https://raw.githubusercontent.com/kaitwalla/swoops-control/main/setup.sh?${timestamp}`
