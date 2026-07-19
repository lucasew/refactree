class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


class HolderA:
    h: BoxA

    def __init__(self, h: BoxA):
        self.h = h


class HolderB:
    h: BoxB

    def __init__(self, h: BoxB):
        self.h = h


class OuterA:
    h: HolderA

    def __init__(self, h: HolderA):
        self.h = h


class OuterB:
    h: HolderB

    def __init__(self, h: HolderB):
        self.h = h


def use_nested(oa: OuterA, ob: OuterB) -> int:
    return oa.h.h.get().run() + ob.h.h.get().run()


def use_assign(oa: OuterA, ob: OuterB) -> int:
    ha = oa.h
    hb = ob.h
    return ha.h.get().run() + hb.h.get().run()


def use_preserves_b(ob: OuterB) -> int:
    return ob.h.h.get().run()
