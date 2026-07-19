class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_match_unannotated():
    aa = [[A()]]
    bb = [[B()]]
    match aa:
        case [[xa, *_], *_]:
            r = xa.execute()
    match bb:
        case [[xb, *_], *_]:
            r += xb.run()
    return r


def use_match_unannotated_row():
    aa = [[A()]]
    bb = [[B()]]
    match aa:
        case [rowa, *_]:
            r = rowa[0].execute()
    match bb:
        case [rowb, *_]:
            r += rowb[0].run()
    return r


def use_match_unannotated_as():
    aa = [[A()]]
    bb = [[B()]]
    match aa:
        case [[xa as x, *_], *_]:
            r = x.execute()
    match bb:
        case [[xb as y, *_], *_]:
            r += y.run()
    return r


def use_match_unannotated_tuple():
    aa = ((A(),),)
    bb = ((B(),),)
    match aa:
        case ((ta, *_), *_):
            r = ta.execute()
    match bb:
        case ((tb, *_), *_):
            r += tb.run()
    return r


def use_index_unannotated():
    aa = [[A()]]
    bb = [[B()]]
    return aa[0][0].execute() + bb[0][0].run()


def use_for_unannotated():
    aa = [[A()]]
    bb = [[B()]]
    n = 0
    for ra in aa:
        for a in ra:
            n += a.execute()
    for rb in bb:
        for b in rb:
            n += b.run()
    return n


def use_preserves_b():
    bb = [[B()]]
    match bb:
        case [[xb]]:
            return xb.run()
