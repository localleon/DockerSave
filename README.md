# WIP: DockerSave
Download Container-Images from DockerHub without the Docker Toolchain. DockerSave is a simple binary without any bullshit. 
It uses the offical Docker Registry API (v2) to download each layer individually and compress it into a full fledged image. 

The tool currently only supports `hub.docker.com`. PR's for support for more registries are welcome. Files are getting downloaded into a tmp-Folder (./golayer/) in your current working directory, so make sure you got write access to your directory. After the tool has finished it leaves a .tar file in your directory which contains the image
## Usage
```
dockersave -image alpine/git -tag latest --out alpine.tar
```
This will save the image `alpine/git` into a tar file. Now copy the tar file onto your docker host.
Load the file with `docker load --input ./imagefile.tar` or `podman load --input ./imagefile.tar`

## CI/CD
Each commit is linted and build by a Github Actions Workflow.  

## Limitations
Only support v2 manifests: some registries, like quay.io which only uses v1 manifests, may not work.

## ToDo's / Known Bugs
- Images without a repository (i.e alpine) are not supported. Only full names like `alpine/git`
- Fake layers ID are not calculated the same way than Docker client does (I don't know yet how layer hashes are generated, but it seems deterministic and based on the client)
- make download of layers parallel
- add support for more registries