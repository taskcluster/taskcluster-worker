package gc


// Garbage collector


//TODO: Provide some base implementation of Disposable that is thread-safe!!!

type ResourceManager interface {
  Remove(resource *DisposableResource)
}

type DisposableResource struct {
  resourceReferenceCount uint32
  resourceLastUsed time.Time
  manager *ResourceManager
}

func (r *DisposableResource) Acquire() {
  r.resourceReferenceCount += 1
}

func (r *DisposableResource) Release() {
  r.resourceReferenceCount -= 1
  r.resourceLastUsed = time.Now()
}

func (r *DisposableResource) LastUsed() time.Time {
  return r.resourceLastUsed
}



type GarbageCollector struct {
  resources []*Disposable
  resourceLock sync.Mutex
}

func (gc *GarbageCollector) Register(resource *Disposable) {
  gc.resourceLock.Lock()
  defer gc.resourceLock.Unlock()
  gc.resources = append(gc.resources, resource)
}

func (gc *GarbageCollector) Collect() {
  gc.resourceLock.Lock()
  defer gc.resourceLock.Unlock()
  // TODO iterate over gc.resources and dispose some things...
}



// Resource manager example:

struct CacheManager {
  GetCacheFolder(cacheFolderName string) *CacheFolder

  // Internal things
}

struct CacheFolder {
  // Internal things
  cacheFolderName string
  path string
}

func (c *CacheFolder) CacheFolderName() string {
  return c.cacheFolderName
}

func (c *CacheFolder) Path() string {
  return c.path
}




