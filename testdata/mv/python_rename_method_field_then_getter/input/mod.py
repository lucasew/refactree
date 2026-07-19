class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class HolderA:
    a: A

    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class HolderB:
    b: B

    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


class OuterA:
    h: HolderA

    def __init__(self, h: HolderA):
        self.h = h


class OuterB:
    h: HolderB

    def __init__(self, h: HolderB):
        self.h = h


def use(oa: OuterA, ob: OuterB) -> int:
    return oa.h.get().run() + ob.h.get().run()


def use_assign(oa: OuterA, ob: OuterB) -> int:
    ha = oa.h
    hb = ob.h
    return ha.get().run() + hb.get().run()


def use_preserves_b(ob: OuterB) -> int:
    return ob.h.get().run()
