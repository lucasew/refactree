type BoxAlias = list[int]

class Crate:
    pass

def use(x: BoxAlias) -> Crate:
    return Crate()
