class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_sorted(items: list[A]):
    sorted(items, key=lambda x: x.execute())


def use_sorted_b(items: list[B]):
    sorted(items, key=lambda y: y.run())


def use_min(items: list[A]):
    return min(items, key=lambda x: x.execute())


def use_min_b(items: list[B]):
    return min(items, key=lambda y: y.run())


def use_max(items: list[A]):
    return max(items, key=lambda x: x.execute())


def use_max_b(items: list[B]):
    return max(items, key=lambda y: y.run())


def use_sort(items: list[A]):
    items.sort(key=lambda x: x.execute())


def use_sort_b(items: list[B]):
    items.sort(key=lambda y: y.run())


def use_map(items: list[A]):
    return list(map(lambda x: x.execute(), items))


def use_map_b(items: list[B]):
    return list(map(lambda y: y.run(), items))


def use_filter(items: list[A]):
    return list(filter(lambda x: x.execute(), items))


def use_filter_b(items: list[B]):
    return list(filter(lambda y: y.run(), items))


def use_sorted_assigned():
    xs = [A()]
    sorted(xs, key=lambda x: x.execute())
    ys = [B()]
    sorted(ys, key=lambda y: y.run())


def use_sorted_literal():
    sorted([A()], key=lambda x: x.execute())
    sorted([B()], key=lambda y: y.run())


def use_sorted_wrapper(items: list[A]):
    sorted(list(items), key=lambda x: x.execute())
