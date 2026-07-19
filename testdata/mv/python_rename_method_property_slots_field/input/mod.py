class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    __slots__ = ("a",)
    a: A

    def __init__(self, a: A):
        self.a = a

    @property
    def item(self) -> A:
        return self.a


class BoxB:
    __slots__ = ("b",)
    b: B

    def __init__(self, b: B):
        self.b = b

    @property
    def item(self) -> B:
        return self.b


def use_slots(ba: BoxA, bb: BoxB) -> int:
    return ba.a.run() + bb.b.run()


def use_property(ba: BoxA, bb: BoxB) -> int:
    return ba.item.run() + bb.item.run()


def use_ctor() -> int:
    return BoxA(A()).a.run() + BoxB(B()).b.run()


def use_property_ctor() -> int:
    return BoxA(A()).item.run() + BoxB(B()).item.run()


def use_preserves_b(bb: BoxB) -> int:
    return bb.b.run() + bb.item.run()
