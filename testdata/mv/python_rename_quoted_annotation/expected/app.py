from mod import Crate

def use(b: "Crate") -> "Crate":
    return b.helper()
