from itertools import chain


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self):
        self.a = A()

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self):
        self.b = B()

    def get(self) -> B:
        return self.b


# Class regression — listcomp/genexp/wrapper match subjects already solid.
def use_match_listcomp_class() -> int:
    n = 0
    match [A() for _ in range(1)]:
        case [x]:
            n += x.run()
    match [B() for _ in range(1)]:
        case [y]:
            n += y.run()
    return n


def use_match_list_genexp_class() -> int:
    n = 0
    match list(A() for _ in range(1)):
        case [x]:
            n += x.run()
    match list(B() for _ in range(1)):
        case [y]:
            n += y.run()
    return n


def use_match_tuple_genexp_class() -> int:
    n = 0
    match tuple(A() for _ in range(1)):
        case (x,):
            n += x.run()
    match tuple(B() for _ in range(1)):
        case (y,):
            n += y.run()
    return n


def use_match_list_map_class() -> int:
    n = 0
    match list(map(lambda z: z, [A()])):
        case [x]:
            n += x.run()
    match list(map(lambda z: z, [B()])):
        case [y]:
            n += y.run()
    return n


def use_match_list_filter_class() -> int:
    n = 0
    match list(filter(None, [A()])):
        case [x]:
            n += x.run()
    match list(filter(None, [B()])):
        case [y]:
            n += y.run()
    return n


def use_match_list_chain_class() -> int:
    n = 0
    match list(chain([A()])):
        case [x]:
            n += x.run()
    match list(chain([B()])):
        case [y]:
            n += y.run()
    return n


# Method-return comprehension / wrapper match subjects under foreign same-leaf.
def use_match_listcomp_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match [ba.get() for _ in range(1)]:
        case [x]:
            n += x.run()
    match [bb.get() for _ in range(1)]:
        case [y]:
            n += y.run()
    return n


def use_match_list_genexp_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match list(ba.get() for _ in range(1)):
        case [x]:
            n += x.run()
    match list(bb.get() for _ in range(1)):
        case [y]:
            n += y.run()
    return n


def use_match_tuple_genexp_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match tuple(ba.get() for _ in range(1)):
        case (x,):
            n += x.run()
    match tuple(bb.get() for _ in range(1)):
        case (y,):
            n += y.run()
    return n


def use_match_list_map_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match list(map(lambda z: z, [ba.get()])):
        case [x]:
            n += x.run()
    match list(map(lambda z: z, [bb.get()])):
        case [y]:
            n += y.run()
    return n


def use_match_list_filter_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match list(filter(None, [ba.get()])):
        case [x]:
            n += x.run()
    match list(filter(None, [bb.get()])):
        case [y]:
            n += y.run()
    return n


def use_match_list_chain_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match list(chain([ba.get()])):
        case [x]:
            n += x.run()
    match list(chain([bb.get()])):
        case [y]:
            n += y.run()
    return n


# Assigned listcomp already solid via elemOf — regression.
def use_match_assign_listcomp_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    xs = [ba.get() for _ in range(1)]
    ys = [bb.get() for _ in range(1)]
    match xs:
        case [x]:
            n += x.run()
    match ys:
        case [y]:
            n += y.run()
    return n
