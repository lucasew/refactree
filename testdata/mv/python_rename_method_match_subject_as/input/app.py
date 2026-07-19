class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_match_typed_local(a: A, b: B) -> int:
    match a:
        case x as xa:
            r = xa.run()
    match b:
        case y as yb:
            r += yb.run()
    return r


def use_match_subject_typed() -> int:
    a: A = A()
    b: B = B()
    match a:
        case x as xa:
            r = xa.run()
    match b:
        case y as yb:
            r += yb.run()
    return r


def use_match_wildcard_as(a: A, b: B) -> int:
    match a:
        case _ as xa:
            r = xa.run()
    match b:
        case _ as yb:
            r += yb.run()
    return r


def use_preserves_b(b: B) -> int:
    match b:
        case x as yb:
            return yb.run()
