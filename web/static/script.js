(function() {
    'use strict';

    // --- состояние ---
    const state = {
        currentPage: 1,
        selectedTag: '',
        totalPages: 1,
        totalItems: 0,
        allTags: [],
        articles: [],
        loading: false,
    };

    // --- DOM-элементы ---
    const feedContainer = document.getElementById('feedContainer');
    const tagFilterContainer = document.getElementById('tagFilterContainer');
    const clearTagsBtn = document.getElementById('clearTagsBtn');
    const pageNumbersSpan = document.getElementById('pageNumbers');
    const prevPageBtn = document.getElementById('prevPage');
    const nextPageBtn = document.getElementById('nextPage');
    const pageInfoSpan = document.getElementById('pageInfo');
    const itemsInfoSpan = document.getElementById('itemsInfo');
    const totalBadge = document.getElementById('totalBadge');

    // --- работа с API (реальный fetch) ---

    async function fetchFeed(page = 1, tag = '') {
        const params = new URLSearchParams();
        params.append('page', String(page));
        if (tag) {
            params.append('tag', tag);
        }

        const url = `/feed?${params.toString()}`;

        const response = await fetch(url);

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();

        return {
            articles: data.articles || [],
            totalPages: data.totalPages || 1,
            totalItems: data.totalItems || 0,
            allTags: data.allTags || [],
            currentPage: data.currentPage || page,
            selectedTag: data.selectedTag || tag,
        };
    }

    // --- рендеринг ---

    function renderFeed(data) {
        const { articles, totalPages, totalItems, allTags, currentPage, selectedTag } = data;

        state.articles = articles;
        state.totalPages = totalPages;
        state.totalItems = totalItems;
        state.allTags = allTags;
        state.currentPage = currentPage;
        state.selectedTag = selectedTag;

        // 1. Рендерим карточки
        if (!articles || articles.length === 0) {
            feedContainer.innerHTML = `<div class="status">📭 Новостей не найдено</div>`;
        } else {
            feedContainer.innerHTML = articles.map(article => `
                <div class="article-card">
                    <div class="article-title">${escapeHtml(article.title)}</div>
                    <div class="article-content">${escapeHtml(article.content)}</div>
                    <div class="article-tags">
                        ${(article.tags || []).map(tag => `<span class="article-tag">${escapeHtml(tag)}</span>`).join('')}
                    </div>
                </div>
            `).join('');
        }

        // 2. Рендерим теги-фильтры
        renderTagFilters(allTags, selectedTag);

        // 3. Рендерим пагинацию
        renderPagination(currentPage, totalPages);

        // 4. Обновляем информацию
        totalBadge.textContent = `${totalItems} нов.`;
        pageInfoSpan.textContent = `страница ${currentPage} из ${totalPages}`;
        itemsInfoSpan.textContent = `${articles.length} из ${totalItems} новостей`;
    }

    function renderTagFilters(allTags, selectedTag) {
        if (!allTags || allTags.length === 0) {
            tagFilterContainer.innerHTML = '<span style="color:#8a9aa8; font-size:0.9rem;">нет тегов</span>';
            return;
        }

        tagFilterContainer.innerHTML = allTags.map(tag => `
            <button class="tag-btn ${tag === selectedTag ? 'active' : ''}" data-tag="${escapeHtml(tag)}">${escapeHtml(tag)}</button>
        `).join('');

        document.querySelectorAll('.tag-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                const tag = this.dataset.tag;
                if (state.selectedTag === tag) {
                    loadFeed(1, '');
                } else {
                    loadFeed(1, tag);
                }
            });
        });
    }

    function renderPagination(currentPage, totalPages) {
        prevPageBtn.disabled = currentPage <= 1;
        nextPageBtn.disabled = currentPage >= totalPages;

        let pagesHtml = '';
        const maxVisible = 5;
        let startPage = Math.max(1, currentPage - Math.floor(maxVisible / 2));
        let endPage = Math.min(totalPages, startPage + maxVisible - 1);

        if (endPage - startPage < maxVisible - 1) {
            startPage = Math.max(1, endPage - maxVisible + 1);
        }

        for (let i = startPage; i <= endPage; i++) {
            const active = i === currentPage ? 'active' : '';
            pagesHtml += `<button class="page-btn ${active}" data-page="${i}">${i}</button>`;
        }

        pageNumbersSpan.innerHTML = pagesHtml;

        document.querySelectorAll('#pageNumbers .page-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                const page = parseInt(this.dataset.page);
                if (page !== state.currentPage) {
                    loadFeed(page, state.selectedTag);
                }
            });
        });
    }

    // --- загрузка данных ---

    async function loadFeed(page, tag) {
        if (state.loading) return;

        state.loading = true;
        feedContainer.innerHTML = `<div class="loader">⏳ Загрузка...</div>`;

        try {
            const data = await fetchFeed(page, tag);
            renderFeed(data);
        } catch (err) {
            console.error('Ошибка загрузки:', err);
            feedContainer.innerHTML = `
                <div class="status">
                    ⚠️ Ошибка загрузки данных<br>
                    <small style="color:#999;">${escapeHtml(err.message)}</small>
                </div>
            `;
        } finally {
            state.loading = false;
        }
    }

    // --- обработчики пагинации ---

    prevPageBtn.addEventListener('click', function() {
        if (state.currentPage > 1) {
            loadFeed(state.currentPage - 1, state.selectedTag);
        }
    });

    nextPageBtn.addEventListener('click', function() {
        if (state.currentPage < state.totalPages) {
            loadFeed(state.currentPage + 1, state.selectedTag);
        }
    });

    // --- сброс фильтра ---

    clearTagsBtn.addEventListener('click', function() {
        if (state.selectedTag !== '') {
            loadFeed(1, '');
        }
    });

    // --- утилиты ---

    function escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // --- инициализация ---

    loadFeed(1, '');
})();