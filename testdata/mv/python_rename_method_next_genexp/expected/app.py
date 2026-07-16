class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_next_gen(items: list[A]):
    a = next(x for x in items)
    a.execute()


def use_next_gen_b(items: list[B]):
    b = next(x for x in items)
    b.run()


def use_next_gen_paren(items: list[A]):
    a = next((x for x in items))
    a.execute()


def use_next_gen_filter(items: list[A]):
    a = next(x for x in items if x)
    a.execute()


def use_next_gen_assigned():
    xs = [A()]
    a = next(x for x in xs)
    a.execute()
    ys = [B()]
    b = next(x for x in ys)
    b.run()


def use_next_gen_literal():
    a = next(x for x in [A()])
    a.execute()
    b = next(x for x in [B()])
    b.run()


def use_next_gen_nested(items: list[A]):
    a = next(x for x in list(items))
    a.execute()
