// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

/**
 * @title sort given array using Quick sort.
 * @author [Priyda](https://github.com/priyda)
 */
library QuickSort {
    function sort(uint128[] memory data, uint256 lValue, uint256 rValue) public view returns (uint128[] memory) {
        quickSort(data, int256(lValue), int256(rValue));
        return data;
    }

    /** Quicksort is a sorting algorithm based on the divide and conquer approach **/
    function quickSort(
        uint128[] memory _arr,
        int256 left,
        int256 right
    ) internal view {
        int256 i = left;
        int256 j = right;
        if (i == j) return;
        uint256 pivot = _arr[uint256(left + (right - left) / 2)];
        while (i <= j) {
            while (_arr[uint256(i)] < pivot) i++;
            while (pivot < _arr[uint256(j)]) j--;
            if (i <= j) {
                (_arr[uint256(i)], _arr[uint256(j)]) = (
                    _arr[uint256(j)],
                    _arr[uint256(i)]
                );
                i++;
                j--;
            }
        }
        if (left < j) quickSort(_arr, left, j);
        if (i < right) quickSort(_arr, i, right);
    }
}
