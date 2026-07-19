class A:
    def execute(self) -> int:
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

    def get(self) -> A:
        return self.a

    def self(self) -> "BoxA":
        return self


class BoxB:
    __slots__ = ("b",)
    b: B

    def __init__(self, b: B):
        self.b = b

    @property
    def item(self) -> B:
        return self.b

    def get(self) -> B:
        return self.b

    def self(self) -> "BoxB":
        return self


def use_get(ba: BoxA, bb: BoxB) -> int:
    return ba.get().execute() + bb.get().run()


def use_self(ba: BoxA, bb: BoxB) -> int:
    return ba.self().item.execute() + bb.self().item.run()


def use_self_get(ba: BoxA, bb: BoxB) -> int:
    return ba.self().get().execute() + bb.self().get().run()


def use_preserves_b(bb: BoxB) -> int:
    return bb.get().run() + bb.self().item.run()
