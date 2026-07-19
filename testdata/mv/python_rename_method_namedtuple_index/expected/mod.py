from collections import namedtuple


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


Box = namedtuple("Box", ["a", "b"])


def use_direct() -> int:
    box = Box(A(), B())
    return box[0].execute() + box[1].run()


def use_var() -> int:
    box = Box(A(), B())
    xa = box[0]
    xb = box[1]
    return xa.execute() + xb.run()


def use_walrus() -> int:
    box = Box(A(), B())
    if (xa := box[0]):
        return xa.execute()
    if (xb := box[1]):
        return xb.run()
    return 0


def use_param(box: Box) -> int:
    return box[0].execute() + box[1].run()


def use_preserves_b() -> int:
    box = Box(A(), B())
    return box[1].run() + box.b.run()
