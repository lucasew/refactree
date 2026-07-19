from collections import namedtuple


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


Box = namedtuple("Box", ["a"])


def use_asdict_sub():
    ba = Box(A())
    bb = Box(B())
    return ba._asdict()["a"].run() + bb._asdict()["a"].run()


def use_asdict_get():
    ba = Box(A())
    bb = Box(B())
    return ba._asdict().get("a").run() + bb._asdict().get("a").run()


def use_asdict_assign():
    ba = Box(A())
    bb = Box(B())
    da = ba._asdict()
    db = bb._asdict()
    return da["a"].run() + db["a"].run()


def use_replace():
    ba = Box(A())
    bb = Box(B())
    return ba._replace(a=A()).a.run() + bb._replace(a=B()).a.run()


def use_replace_field():
    ba = Box(B())
    ra = ba._replace(a=A())
    return ra.a.run()


def use_make():
    return Box._make([A()]).a.run() + Box._make([B()]).a.run()


def use_make_index():
    return Box._make([A()])[0].run() + Box._make([B()])[0].run()


def use_make_assign():
    ba = Box._make([A()])
    bb = Box._make([B()])
    return ba.a.run() + bb.a.run()


def use_preserves_b():
    bb = Box(B())
    return bb._asdict()["a"].run() + Box._make([B()]).a.run()
