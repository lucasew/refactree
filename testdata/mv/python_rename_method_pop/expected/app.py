class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_pop(items: list[A]):
    a = items.pop()
    a.execute()


def use_pop_b(items: list[B]):
    b = items.pop()
    b.run()


def use_pop_index(items: list[A]):
    a = items.pop(0)
    a.execute()


def use_dict_pop(d: dict[str, A]):
    a = d.pop("k")
    a.execute()


def use_dict_pop_b(d: dict[str, B]):
    b = d.pop("k")
    b.run()


def use_pop_assigned():
    xs = [A()]
    a = xs.pop()
    a.execute()
    ys = [B()]
    b = ys.pop()
    b.run()


def use_pop_wrapper(items: list[A]):
    a = list(items).pop()
    a.execute()


def use_pop_wrapper_assigned(items: list[A]):
    xs = list(items)
    a = xs.pop()
    a.execute()
