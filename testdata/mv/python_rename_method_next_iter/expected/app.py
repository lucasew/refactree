class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_next_iter(items: list[A]):
    a = next(iter(items))
    a.execute()


def use_next_iter_b(items: list[B]):
    b = next(iter(items))
    b.run()


def use_next_direct(items: list[A]):
    a = next(items)
    a.execute()


def use_next_reversed(items: list[A]):
    a = next(reversed(items))
    a.execute()


def use_next_filter(items: list[A]):
    a = next(filter(None, items))
    a.execute()


def use_next_assigned():
    xs = [A()]
    a = next(iter(xs))
    a.execute()
    ys = [B()]
    b = next(iter(ys))
    b.run()


def use_next_literal():
    a = next(iter([A()]))
    a.execute()
    b = next(iter([B()]))
    b.run()


def use_next_nested(items: list[A]):
    a = next(iter(list(items)))
    a.execute()
