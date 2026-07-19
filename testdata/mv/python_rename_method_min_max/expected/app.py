class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_min(items: list[A]):
    a = min(items)
    a.execute()


def use_max(items: list[A]):
    a = max(items)
    a.execute()


def use_min_b(items: list[B]):
    b = min(items)
    b.run()


def use_max_b(items: list[B]):
    b = max(items)
    b.run()


def use_min_key(items: list[A]):
    a = min(items, key=lambda x: 0)
    a.execute()


def use_max_key(items: list[A]):
    a = max(items, key=lambda x: 0)
    a.execute()


def use_min_assigned():
    xs = [A()]
    a = min(xs)
    a.execute()
    ys = [B()]
    b = min(ys)
    b.run()


def use_min_wrapper(items: list[A]):
    a = min(list(items))
    a.execute()


def use_min_reversed(items: list[A]):
    a = min(reversed(items))
    a.execute()


def use_min_literal():
    a = min([A()])
    a.execute()
    b = min([B()])
    b.run()
