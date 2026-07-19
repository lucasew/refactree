class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_or_empty(aa: list[A] | None, bb: list[B] | None):
    return (aa or [])[0].execute() + (bb or [])[0].run()


def use_or_empty_assign(aa: list[A] | None, bb: list[B] | None):
    xs = aa or []
    ys = bb or []
    return xs[0].execute() + ys[0].run()


def use_or_empty_for(aa: list[A] | None, bb: list[B] | None):
    s = 0
    for a in aa or []:
        s += a.execute()
    for b in bb or []:
        s += b.run()
    return s
