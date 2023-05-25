# Failure in user update/validation

User update or validation code that fails will be translated into an error on
the caller side.

# Detailed spec

The error received by the caller will be through the handle.Get call and will
contain the string contents of the panic/exception message. Panics/Exceptions out of validation
handlers are equivalent to rejections from a durability perspective while panics/exceptions
from the main execution function are the equivalent of panicing/throwing in a workflow task.
