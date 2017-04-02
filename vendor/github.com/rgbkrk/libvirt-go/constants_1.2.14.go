// +build libvirt.1.2.14

package libvirt

/*
#cgo LDFLAGS: -lvirt
#include <libvirt/libvirt.h>
#include <libvirt/virterror.h>
#include <stdlib.h>
*/
import "C"

// virStorageVolCreateFlags
const (
	VIR_STORAGE_VOL_CREATE_REFLINK = C.VIR_STORAGE_VOL_CREATE_REFLINK
)
// virDomainBlockCopyFlags
const (
	VIR_DOMAIN_BLOCK_COPY_SHALLOW   = C.VIR_DOMAIN_BLOCK_COPY_SHALLOW
	VIR_DOMAIN_BLOCK_COPY_REUSE_EXT = C.VIR_DOMAIN_BLOCK_COPY_REUSE_EXT
)
// virDomainBlockCopyParameterObjects
const (
	VIR_DOMAIN_BLOCK_COPY_GRANULARITY = C.VIR_DOMAIN_BLOCK_COPY_GRANULARITY
	VIR_DOMAIN_BLOCK_COPY_BANDWIDTH   = C.VIR_DOMAIN_BLOCK_COPY_BANDWIDTH
	VIR_DOMAIN_BLOCK_COPY_BUF_SIZE    = C.VIR_DOMAIN_BLOCK_COPY_BUF_SIZE
)

// virDomainBlockJobInfoFlags
const (
	VIR_DOMAIN_BLOCK_JOB_INFO_BANDWIDTH_BYTES = C.VIR_DOMAIN_BLOCK_JOB_INFO_BANDWIDTH_BYTES
)