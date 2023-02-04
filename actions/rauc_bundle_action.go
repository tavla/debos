/*
RAUC Bundle Action

Create a RAUC bundle from filesystem(s).

	# Yaml syntax:
	- action: rauc-bundle
	  TODO

Mandatory properties:

- bundle -- TODO
- cert -- TODO
- key -- TODO

Optional properties:

- TODO

# TODO additional bits inside the rauc manifest file

TODO require rauc and squashfs-tools
*/
package actions

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"syscall"

	"github.com/go-debos/debos"
	"gopkg.in/ini.v1"
)

type BundleImage struct {
	Name      string
	Partition string
}

type RaucBundleAction struct {
	debos.BaseAction `yaml:",inline"`
	Bundle           string
	Cert             string
	Key              string
	Metadata         map[string]map[string]string
	Images           []BundleImage
}

// Create an image with the contents of a device node
func (rb *RaucBundleAction) copyDeviceToImage(context *debos.DebosContext, device string, image string) error {
	log.Printf("Copying %s to %s", device, image)

	log.Printf("here22\n")

	/* Actually copy the data from the device in small chunks */
	deviceFd, err := os.Open(device)
	if err != nil {
		return err
	}
	defer deviceFd.Close()

	log.Printf("here1\n")
	imageFd, err := os.Create(image)
	if err != nil {
		return err
	}
	defer imageFd.Close()

	log.Printf("here2\n")

	// TODO does it try to copy whole image into memory or does it do it properly ?
	if _, err := io.Copy(imageFd, deviceFd); err != nil {
		return err
	}

	// TODO benchmark a suitable value for bytes
	/*var bytes int64 = 8 * 1024 * 1024
	for {
		written, err := io.CopyN(imageFd, deviceFd, bytes)
		if err != nil {
			return err
		}

		if written < bytes {
			break
		}
	}*/

	/* TODO Think about mounting the device again with the same options */

	//return fmt.Errorf("blah")
	return nil
}

func (rb *RaucBundleAction) Verify(context *debos.DebosContext) error {
	if len(rb.Bundle) == 0 {
		return fmt.Errorf("bundle cannot be empty")
	}

	if len(rb.Cert) == 0 {
		return fmt.Errorf("cert cannot be empty")
	}

	if len(rb.Key) == 0 {
		return fmt.Errorf("key cannot be empty")
	}

	if len(rb.Images) == 0 {
		return fmt.Errorf("bundle must contain at least one image")
	}

	/* TODO verify required bundle metadata keys exist */
	// rb.Manifest: map[hooks:map[filename:bundle-hooks hooks:install-check] update:map[compatible:Compatible1 version:Version1]]
	fmt.Printf("manifest: %v\n", rb.Metadata)

	/* Verify the bundle images */
	for i, image := range rb.Images {
		if len(image.Name) == 0 {
			return fmt.Errorf("image name cannot be empty")
		}

		if len(image.Partition) == 0 {
			return fmt.Errorf("image partition cannot be empty")
		}

		/* Check for duplicate images */
		for j := i + 1; j < len(rb.Images); j++ {
			if rb.Images[j].Name == image.Name {
				return fmt.Errorf("image %s already exists", image.Name)
			}
		}

		/* TODO Verify required metadata keys exist */

		/* Check the partition exists */
		foundPartition := false
		for _, p := range context.ImagePartitions {
			if p.Name == image.Partition {
				foundPartition = true
			}
		}
		if !foundPartition {
			return fmt.Errorf("partition %s for image %s not found", image.Partition, image.Name)
		}
	}

	return nil
}

// TODO what happens if the action is called without/before ImagePartitionAction ?
func (rb *RaucBundleAction) Run(context *debos.DebosContext) error {
	rb.LogStart()

	// TODO unmount _all_ mountpoints then remount them at the end

	/* TODO See if creating bundle in artifactdir has a performance hit */

	/* Remove RAUC bundle if it already exists */
	bundle := path.Join(context.Artifactdir, rb.Bundle)
	if _, err := os.Stat(bundle); err == nil {
		if err = os.Remove(bundle); err != nil {
			return err
		}
	}

	/* Create a temporary directory to store intermediate filesystem images */
	workdir, err := os.MkdirTemp(context.Scratchdir, "rauc")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workdir)

	bundlefilesdir := path.Join(workdir, "bundlefiles")
	if err = os.Mkdir(bundlefilesdir, os.ModePerm); err != nil {
		return err
	}

	/* Create RAUC manifest */
	manifest := ini.Empty()
	manifestUpdateSection, err := manifest.NewSection("update")
	if err != nil {
		return err
	}

	if _, err = manifestUpdateSection.NewKey("compatible", "TODO"); err != nil {
		return err
	}

	if _, err = manifestUpdateSection.NewKey("version", "TODO"); err != nil {
		return err
	}

	/* TODO Generate filesystem images */
	for _, image := range rb.Images {
		/* Find the device node to take an image from */
		var devicePath string
		for _, p := range context.ImagePartitions {
			if p.Name == image.Partition {
				devicePath = p.DevicePath
				break
			}
		}
		log.Printf("devicepath=%s", devicePath)
		if devicePath == "" {
			return fmt.Errorf("could not resolve partition %s", image.Partition)
		}

		/* Unmount the device */
		// TODO need to unmount in mount order e.g. make sure /data is unmounted before /
		for _, m := range context.ImageMounts {
			if m.DevicePath != devicePath {
				continue
			}

			// TODO do it
			fmt.Printf("mountpoint: %s\n", m.MountPoint)
			context.Unmount(m.MountPoint)
			/* Unmount the device */
			// TODO see what the flags should be
			// TODO see if we can unmount multiple mounts

			// TODO check it is mounted before unmounting
			// This really is a unmount2 call
			if err := syscall.Unmount(m.MountPoint, 0); err != nil {
				log.Printf("broken\n")
				// TODO broken here
				return err
			}
		}

		/* Generate an image from the partition contents */
		imagePath := path.Join(bundlefilesdir, image.Partition+".img")
		if err = rb.copyDeviceToImage(context, devicePath, imagePath); err != nil {
			return err
		}

		// TODO mount it again?
	}

	manifest.SaveTo(path.Join(bundlefilesdir, "manifest.raucm"))

	/* TODO Call RAUC to actually create the RAUC bundle */

	/* TODO Add other files into the RAUC bundle */

	/* TODO Export bundle into output directory */

	/* TODO add ability to convert to casync bundle ? */

	/*

	   EFI_DEVICE_SUFFIX="$1"
	   VERITY_DEVICE_SUFFIX="$2"
	   ROOTFS_DEVICE_SUFFIX="$3"
	   RAUC_COMPATIBLE="$4"
	   RAUC_VERSION="$5"
	   IMGNAME="$6"

	   # Create a working directory (see below for why we use /scratch)

	   [ -d "/scratch" ] || fail "No /scratch directory"
	   WORKDIR="/scratch/rauc-workdir"
	   rm -fr "$WORKDIR"
	   mkdir "$WORKDIR"
	   trap "rm -fr $WORKDIR" EXIT

	   # Extract the following block devices: efi, verity, rootfs

	   # We will copy data straight from block devices, it is VERY IMPORTANT
	   # to ensure that everything has been synced, otherwise we'll get
	   # corrupted data. Calling sync doesn't seem to be enough. What we
	   # actually want is to unmount these partitions, to ensure that all
	   # the data is synced to the block device. The problem with umounting
	   # is that it will make debos unhappy in the end. So instead we remount
	   # read-only: it seems to do the job and it keeps debos happy.

	   BUNDLEFILESDIR="$WORKDIR/bundle-files"
	   mkdir "$BUNDLEFILESDIR"

	   partprobe "$IMAGE"

	   EFIDEV="$(realpath "${IMAGE}${EFI_DEVICE_SUFFIX}")"
	   [ "$EFIDEV" ] || fail "Failed to resolve efi device"
	   VERITYDEV="$(realpath "${IMAGE}${VERITY_DEVICE_SUFFIX}")"
	   [ "$VERITYDEV" ] || fail "Failed to resolve verity device"
	   ROOTFSDEV="$(realpath "${IMAGE}${ROOTFS_DEVICE_SUFFIX}")"
	   [ "$ROOTFSDEV" ] || fail "Failed to resolve rootfs device"

	   echo "Extracting EFI partition, device='$EFIDEV'"
	   EFIIMG="$BUNDLEFILESDIR/efi.img"
	   mount -v -o remount,ro $EFIDEV
	   cat $EFIDEV > $EFIIMG

	   echo "Extracting verity partition, device='$VERITYDEV'"
	   VERITYIMG="$BUNDLEFILESDIR/verity.img"
	   cat $VERITYDEV > $VERITYIMG

	   echo "Extracting rootfs partition, device='$ROOTFSDEV'"
	   ROOTFSIMG="$BUNDLEFILESDIR/rootfs.img"
	   mount -v -o remount,ro $ROOTFSDEV
	   cat $ROOTFSDEV > $ROOTFSIMG

	   # Copy any extra files into the rauc bundle
	   BUNDLEFILES_EXTRA="$RECIPEDIR/overlays/rauc-bundle"
	   [ -d "$BUNDLEFILES_EXTRA" ] && cp ${BUNDLEFILES_EXTRA}/* "${BUNDLEFILESDIR}"

	   # Create the rauc manifest

	   cat << EOF > "$BUNDLEFILESDIR/manifest.raucm"
	   [update]
	   compatible=$RAUC_COMPATIBLE
	   version=$RAUC_VERSION

	   [image.rootfs]
	   filename=$(basename $ROOTFSIMG)
	   hooks=pre-install

	   [image.verity]
	   filename=$(basename $VERITYIMG)

	   [image.efi]
	   filename=$(basename $EFIIMG)

	   [hooks]
	   filename=bundle-hooks
	   hooks=install-check
	   EOF

	   # Create the native rauc bundle

	   KEYDIR="$RECIPEDIR/keyring/rauc-keys"

	   echo "Creating native RAUC bundle ..."
	   rauc bundle \
	       --cert $KEYDIR/cert-staging.pem \
	       --key  $KEYDIR/key-staging.pem \
	       "$BUNDLEFILESDIR" \
	       "$WORKDIR/native-bundle.raucb"
	   rauc info \
	       --keyring $KEYDIR/cert-staging.pem \
	       "$WORKDIR/native-bundle.raucb" \
	       > "$WORKDIR/native-bundle.info"

	   rm -fr "$BUNDLEFILESDIR"

	   # Convert to a casync bundle

	   # Here it gets a bit tricky: this operation will require a lot of storage
	   # in tmp directories. By default, rauc will unsquash the bundle in '/tmp',
	   # while casync will create thousands of files in '/var/tmp'. In both cases,
	   # this can't work because there's not enough space available in '/'
	   # (remember, we're within a fakemachine).
	   #
	   # We can solve that by setting TMPDIR, which is honored by both rauc and
	   # casync. The best location to use for this temporary data is `/scratch`
	   # (ie. WORKDIR), as we can control is size when we invoke debos, and also
	   # because it's backed by a proper filesystem (ext4).
	   #
	   # Don't even think about using RECIPEDIR here, because it's backed by a 9pfs,
	   # a network filesystem that is slow, doesn't have all the features required,
	   # an is also buggy. Using it as a work directory is just silly.

	   TMPDIR="$WORKDIR/tmp"
	   mkdir "$TMPDIR"
	   export TMPDIR

	   echo "Converting to CASync bundle ..."
	   rauc convert \
	       --cert $KEYDIR/cert-staging.pem \
	       --key  $KEYDIR/key-staging.pem \
	       --keyring $KEYDIR/cert-staging.pem \
	       --casync-args="--chunk-size=64000:256000:448000" \
	       $WORKDIR/native-bundle.raucb \
	       $WORKDIR/casync-bundle.raucb
	   rauc info \
	       --keyring $KEYDIR/cert-staging.pem \
	       $WORKDIR/casync-bundle.raucb \
	       > $WORKDIR/casync-bundle.info

	   # Debugging things? Uncomment the following if you need to inspect
	   # everything that was generated by the rauc commands above.

	   #tar -C "$WORKDIR" -cf "$WORKDIR/casync-bundle.castr.tar casync-bundle.castr
	   #rm -fr casync-bundle.castr
	   #mkdir "$RECIPEDIR/$IMGNAME.rauc"
	   #cp $WORKDIR/native-bundle.* "$RECIPEDIR/$IMGNAME.rauc"
	   #cp $WORKDIR/casync-bundle.* "$RECIPEDIR/$IMGNAME.rauc"
	   #exit 0

	   # Move the result in RECIPEDIR. We only keep the rauc+casync bundle files.

	   # Here it gets a bit painful. A lot painful actually.
	   #
	   # Let's start with 9pfs issues:
	   #
	   # (1) 9pfs can't preserve ownership of files. Commands like `cp` or `mv`
	   #     display a warning, however they return 0, even in the case where the
	   #     ownership was not preserved. Apparently, this is due to the QEmu virtfs
	   #     option `security_model` which is set to `none`.
	   #     - <https://unix.stackexchange.com/a/131181/105794>
	   #     - <https://wiki.qemu.org/Documentation/9psetup>
	   #
	   # (2) A bug prevents the copy of files that are read-only. Somehow, 9pfs
	   #     doesn't handle `openat()` with options `O_WRONLY|O_CREAT|O_EXCL, 0444`.
	   #     This affects both `cp` and `mv`, and results in creating empty files.
	   #     - <https://patchwork.kernel.org/patch/10107901/>
	   #
	   # And now let's see everything that I tried and that failed, in order to copy
	   # the build artifacts from WORKDIR (ext4 backed) to RECIPEDIR (9pfs backed).
	   # Let's remember that the casync store contains thousands of files and dirs,
	   # and all the files are read-only.
	   #
	   # (a) mv
	   #     - warning flood due to (1)
	   #     - failure due to (2)
	   #     - 7 minutes to copy the casync store (with empty files due to (2))
	   #
	   # (b) cp --no-preserve=ownership
	   #     - no warning
	   #     - failure due to (2)
	   #     - (didn't time it)
	   #
	   # (c) rsync -rlptD
	   #     - no warning
	   #     - no failure, it actually works
	   #     - 20 minutes to copy the casync store!
	   #
	   # So at this point, the conclusion is that:
	   # - 9pfs is almost unusable due to limitation and bug
	   # - if those were improved/fixed, it would still be too slow to be usable
	   #
	   # The only way out I see is to tar the casync store, and copy the archive.
	   # And even though, don't expect to untar it on the 9pfs: it takes 6 minutes,
	   # and fails anyway due to (2).

	   RAUC_BUNDLE="$IMGNAME.raucb"
	   CASYNC_STORE="$IMGNAME.castr"

	   mv "$WORKDIR/casync-bundle.raucb" "$WORKDIR/$RAUC_BUNDLE"
	   mv "$WORKDIR/casync-bundle.castr" "$WORKDIR/$CASYNC_STORE"

	   echo "Archiving the CASync store ..."
	   tar -C "$WORKDIR" -cf "$WORKDIR/$CASYNC_STORE".tar "$CASYNC_STORE"

	   # original script copied into RECIPEDIR
	   #OUTDIR=$RECIPEDIR
	   OUTDIR=$ARTIFACTDIR
	   echo "Moving build artifacts to '$OUTDIR' ..."
	   cp "$WORKDIR/$RAUC_BUNDLE"      "$OUTDIR"
	   cp "$WORKDIR/$CASYNC_STORE".tar "$OUTDIR"

	*/

	cmdline := []string{"rauc", "bundle", "--debug"}

	cmdline = append(cmdline, "--cert="+path.Join(context.RecipeDir, rb.Cert))
	cmdline = append(cmdline, "--key="+path.Join(context.RecipeDir, rb.Key))
	// TODO keyring is optional ? but it checks the cert etc
	cmdline = append(cmdline, "--keyring="+path.Join(context.RecipeDir, rb.Cert))

	cmdline = append(cmdline, bundlefilesdir)

	cmdline = append(cmdline, bundle)

	/*
		TODO regenerate key which does not expire
			openssl req -x509 -newkey rsa:4096 -nodes -keyout test.key.pem -out test.cert.pem -subj "/O=go-debos/CN=debos"

			rauc bundle --cert=<certfile> --key=<keyfile> --keyring=<keyringfile> <input-dir> <output-file>
	*/

	/*
		TODO Allow additional data to be appended to manifest
	*/

	/*
			TODO pkcs11
			RAUC_PKCS11_MODULE
		  --cert='pkcs11:token=rauc;object=autobuilder-1' \
		  --key='pkcs11:token=rauc;object=autobuilder-1' \
	*/

	err = debos.Command{}.Run("rauc", cmdline...)
	if err != nil {
		return err
	}

	/* TODO Run rauc info on the bundle after generation */
	/*cmdline = []string{"rauc", "info", "--debug"}
	cmdline = append(cmdline, "--cert="+path.Join(context.RecipeDir, rb.Cert))
	cmdline = append(cmdline, "--key="+path.Join(context.RecipeDir, rb.Key))
	cmdline = append(cmdline, "--keyring="+path.Join(context.RecipeDir, rb.Cert))
	cmdline = append(cmdline, bundle)
	err = debos.Command{}.Run("rauc", cmdline...)
	if err != nil {
		return err
	}*/

	/* TODO Copy RAUC Bundle to $ARTIFACTDIR - not sure we need to? */
	//os.Move(raucbundle, )

	return nil
}

/* TODO Copy the filesystem AFTER image-partition Cleanup BUT before PostFakemachine... */
func (rb *RaucBundleAction) Cleanup(context *debos.DebosContext) error {
	log.Println("cleanup raucbundleaction")
	return nil
}
